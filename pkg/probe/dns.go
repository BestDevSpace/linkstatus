package probe

import (
	"context"
	"net"
	"sync"
	"time"
)

// DNSProbe probes internet connectivity by resolving a domain via public DNS resolvers.
type DNSProbe struct {
	DNSTargets []string
	DNSDomain  string
	Timeout    time.Duration
}

func NewDNSProbe(targets []string, domain string, timeout time.Duration) *DNSProbe {
	if len(targets) == 0 {
		targets = []string{"8.8.8.8:53", "1.1.1.1:53", "9.9.9.9:53"}
	}
	if domain == "" {
		domain = "google.com"
	}
	return &DNSProbe{DNSTargets: targets, DNSDomain: domain, Timeout: timeout}
}

func (p *DNSProbe) Probe() (Result, error) {
	result := Result{Timestamp: time.Now()}

	if len(p.DNSTargets) == 0 {
		msg := "no DNS targets"
		result.ErrorMessage = &msg
		result.Status = "down"
		result.LatencyMs = 0
		return result, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var totalLatency float64
	var successCount int

	for _, dnsServer := range p.DNSTargets {
		dnsServer := dnsServer
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{Timeout: p.Timeout}
					return d.Dial("udp", dnsServer)
				},
			}
			start := time.Now()
			_, err := r.LookupHost(context.Background(), p.DNSDomain)
			elapsed := time.Since(start).Seconds() * 1000 // to ms
			if err != nil {
				return
			}
			mu.Lock()
			totalLatency += elapsed
			successCount++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if successCount == 0 {
		msg := "all DNS resolvers failed"
		result.ErrorMessage = &msg
		result.Status = "down"
		result.LatencyMs = 0
		return result, nil
	}

	result.PingLatency = nil
	avgLatency := totalLatency / float64(successCount)
	result.DNSLatency = &avgLatency
	result.LatencyMs = avgLatency
	result.Status = "up"

	return result, nil
}
