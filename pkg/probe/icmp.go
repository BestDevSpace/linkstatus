package probe

import (
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// icmpEchoCounter assigns unique (ID, Seq) pairs for concurrent ICMP echo requests.
var icmpEchoCounter atomic.Uint32

// ICMPProbe probes internet connectivity by sending ICMP echo requests.
type ICMPProbe struct {
	Targets []string
	Count   int
	Timeout time.Duration
}

func NewICMPProbe(targets []string, count int, timeout time.Duration) *ICMPProbe {
	if len(targets) == 0 {
		targets = []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"}
	}
	return &ICMPProbe{Targets: targets, Count: count, Timeout: timeout}
}

func (p *ICMPProbe) Probe() (Result, error) {
	result := Result{Timestamp: time.Now()}

	type job struct {
		target string
	}
	var jobs []job
	for _, target := range p.Targets {
		for range p.Count {
			jobs = append(jobs, job{target})
		}
	}
	if len(jobs) == 0 {
		msg := "no ping targets"
		result.ErrorMessage = &msg
		result.Status = "down"
		result.LatencyMs = 0
		return result, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var totalLatency float64
	var successCount int

	for _, j := range jobs {
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			rtt, err := p.pingOne(j.target, p.Timeout)
			if err != nil {
				return
			}
			mu.Lock()
			totalLatency += rtt
			successCount++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if successCount == 0 {
		msg := "all ping targets unreachable"
		result.ErrorMessage = &msg
		result.Status = "down"
		result.LatencyMs = 0
		return result, nil
	}

	avgLatency := totalLatency / float64(successCount)
	result.PingLatency = &avgLatency
	result.LatencyMs = avgLatency
	result.Status = "up"

	return result, nil
}

func (p *ICMPProbe) pingOne(host string, timeout time.Duration) (float64, error) {
	// Try raw ICMP first (requires root/cap_net_raw)
	rtt, err := icmpPing(host, timeout)
	if err != nil {
		// Fallback: try TCP SYN to port 80/443
		return tcpPing(host, []string{"80", "443"}, timeout)
	}
	return rtt, nil
}

func icmpPing(host string, timeout time.Duration) (float64, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	tag := icmpEchoCounter.Add(1)
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   int(uint16(tag) ^ uint16(os.Getpid())),
			Seq:  int(uint16(tag >> 16)),
			Data: []byte("linkstatus"),
		},
	}

	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	deadline := start.Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, err
	}

	if _, err := conn.WriteTo(msgBytes, &net.IPAddr{IP: net.ParseIP(host)}); err != nil {
		return 0, err
	}

	reply := make([]byte, 1500)
	nRead, peer, err := conn.ReadFrom(reply)
	if err != nil {
		return 0, err
	}
	_ = peer

	elapsed := time.Since(start).Seconds() * 1000 // to ms

	rm, err := icmp.ParseMessage(1, reply[:nRead])
	if err != nil {
		return 0, err
	}

	if rm.Type != ipv4.ICMPTypeEchoReply {
		return 0, fmt.Errorf("unexpected ICMP message type: %v", rm.Type)
	}

	return elapsed, nil
}

func tcpPing(host string, ports []string, timeout time.Duration) (float64, error) {
	if len(ports) == 0 {
		return 0, fmt.Errorf("no TCP ports for %s", host)
	}
	var mu sync.Mutex
	var best float64
	ok := false
	var wg sync.WaitGroup
	for _, port := range ports {
		port := port
		wg.Add(1)
		go func() {
			defer wg.Done()
			addr := net.JoinHostPort(host, port)
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err != nil {
				return
			}
			elapsed := time.Since(start).Seconds() * 1000
			_ = conn.Close()
			mu.Lock()
			if !ok || elapsed < best {
				ok = true
				best = elapsed
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if !ok {
		return 0, fmt.Errorf("TCP ping failed for %s on all ports", host)
	}
	return best, nil
}
