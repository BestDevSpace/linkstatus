package worker

import (
	"fmt"
	"time"

	"github.com/BestDevSpace/linkstatus/pkg/probe"
	"github.com/BestDevSpace/linkstatus/pkg/rating"
	"github.com/BestDevSpace/linkstatus/pkg/store"
)

// RunProbe executes one ICMP/DNS probe cycle and persists the result.
// onLog receives human lines (no trailing newline required).
// Returns persisted status ("up" or "down") and an error if the row could not be saved.
func RunProbe(st *store.Store, icmpProbe *probe.ICMPProbe, dnsProbe *probe.DNSProbe, onLog func(string)) (status string, err error) {
	result, err := icmpProbe.Probe()
	if err != nil {
		if onLog != nil {
			onLog(fmt.Sprintf("[%s] ICMP probe error: %v", time.Now().Format("15:04:05"), err))
		}
	}

	if result.Status == "down" {
		dnsResult, err := dnsProbe.Probe()
		if err == nil {
			result = dnsResult
		} else if onLog != nil {
			onLog(fmt.Sprintf("[%s] DNS probe error: %v", time.Now().Format("15:04:05"), err))
		}
	}

	r := rating.Rate(result.LatencyMs)
	logLabel := rating.RatingLabel(r)

	entry := &store.ProbeLogEntry{
		Timestamp: result.Timestamp,
		Status:    result.Status,
		Rating:    r,
		LatencyMs: result.LatencyMs,
	}
	if result.PingLatency != nil {
		entry.PingLatency = result.PingLatency
	}
	if result.DNSLatency != nil {
		entry.DNSLatency = result.DNSLatency
	}
	if result.ErrorMessage != nil {
		entry.ErrorMsg = result.ErrorMessage
	}

	if err := st.InsertEntry(entry); err != nil {
		if onLog != nil {
			onLog(fmt.Sprintf("Error saving log: %v", err))
		}
		return "", err
	}

	statusIcon := "UP"
	if result.Status == "down" {
		statusIcon = "DOWN"
	}

	if onLog != nil {
		onLog(fmt.Sprintf("[%s] %s | Rating: %d/5 (%s) | Latency: %.1fms",
			result.Timestamp.Format("15:04:05"),
			statusIcon,
			r,
			logLabel,
			result.LatencyMs,
		))
	}

	return result.Status, nil
}
