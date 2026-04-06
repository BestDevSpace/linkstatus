package rating

import (
	"math"
	"strings"
)

// Rate calculates a 1-5 rating based on latency in milliseconds.
// Lower latency = higher rating.
func Rate(latencyMs float64) int {
	if latencyMs <= 0 {
		return 1
	}

	thresholds := []float64{20, 50, 100, 300}
	latency := math.Round(latencyMs*100) / 100

	switch {
	case latency <= thresholds[0]:
		return 5
	case latency <= thresholds[1]:
		return 4
	case latency <= thresholds[2]:
		return 3
	case latency <= thresholds[3]:
		return 2
	default:
		return 1
	}
}

// RatingLabel returns a human-readable label for the rating.
func RatingLabel(r int) string {
	switch r {
	case 5:
		return "Excellent"
	case 4:
		return "Good"
	case 3:
		return "Fair"
	case 2:
		return "Poor"
	case 1:
		return "Critical"
	default:
		return "Unknown"
	}
}

// RatingBar returns a visual bar string for the rating.
func RatingBar(r int) string {
	return strings.Repeat("█", r) + strings.Repeat("░", 5-r)
}
