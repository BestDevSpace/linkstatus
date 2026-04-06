package probe

import "time"

type Result struct {
	Status       string // "up" or "down"
	LatencyMs    float64
	PingLatency  *float64
	DNSLatency   *float64
	ErrorMessage *string
	Timestamp    time.Time
}

type Probe interface {
	Probe() (Result, error)
}
