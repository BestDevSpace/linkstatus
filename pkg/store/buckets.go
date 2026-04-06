package store

import (
	"fmt"
	"time"
)

// GetBucketAverageRatings returns exactly n values for [since, until) split into
// n equal buckets (oldest = index 0, newest = n-1).
// Each element is nil when there were no probes in that bucket; otherwise the
// average of rating (1–5) for probes in that bucket.
func (s *Store) GetBucketAverageRatings(since, until time.Time, n int) ([]*float64, error) {
	if n <= 0 {
		return nil, fmt.Errorf("n must be positive")
	}
	if !until.After(since) {
		return nil, fmt.Errorf("until must be after since")
	}
	window := until.Sub(since)
	entries, err := s.GetEntriesSince(since)
	if err != nil {
		return nil, err
	}
	bucketDur := window / time.Duration(n)
	type acc struct {
		sum   float64
		count int
	}
	buckets := make([]acc, n)
	for _, e := range entries {
		if e.Timestamp.Before(since) || !e.Timestamp.Before(until) {
			continue
		}
		idx := int(e.Timestamp.Sub(since) / bucketDur)
		if idx < 0 {
			continue
		}
		if idx >= n {
			idx = n - 1
		}
		buckets[idx].sum += float64(e.Rating)
		buckets[idx].count++
	}
	out := make([]*float64, n)
	for i := range buckets {
		if buckets[i].count == 0 {
			continue
		}
		v := buckets[i].sum / float64(buckets[i].count)
		out[i] = &v
	}
	return out, nil
}
