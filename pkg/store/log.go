package store

import (
	"database/sql"
	"fmt"
	"time"
)

type ProbeLogEntry struct {
	Timestamp   time.Time
	Status      string  // "up" or "down"
	Rating      int     // 1-5
	LatencyMs   float64
	PingLatency *float64
	DNSLatency  *float64
	ErrorMsg    *string
}

type AggregatedStats struct {
	TotalProbes   int
	UpProbes      int
	DownProbes    int
	UptimePercent float64
	AvgLatency    float64
	MinLatency    float64
	MaxLatency    float64
	AvgRating     float64
}

func (s *Store) InsertEntry(entry *ProbeLogEntry) error {
	query := `
		INSERT INTO probe_logs (timestamp, status, rating, latency_ms, ping_latency_ms, dns_latency_ms, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query,
		entry.Timestamp,
		entry.Status,
		entry.Rating,
		entry.LatencyMs,
		ptrOrNil(entry.PingLatency),
		ptrOrNil(entry.DNSLatency),
		ptrOrNilStr(entry.ErrorMsg),
	)
	if err != nil {
		return fmt.Errorf("inserting probe log: %w", err)
	}
	return nil
}

func ptrOrNil(f *float64) interface{} {
	if f == nil {
		return nil
	}
	return *f
}

func ptrOrNilStr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func (s *Store) GetLatestEntry() (*ProbeLogEntry, error) {
	query := `SELECT timestamp, status, rating, latency_ms, ping_latency_ms, dns_latency_ms, error_message
		FROM probe_logs ORDER BY timestamp DESC LIMIT 1`

	var entry ProbeLogEntry
	var pingLatency sql.NullFloat64
	var dnsLatency sql.NullFloat64
	var errorMsg sql.NullString

	err := s.db.QueryRow(query).Scan(
		&entry.Timestamp,
		&entry.Status,
		&entry.Rating,
		&entry.LatencyMs,
		&pingLatency,
		&dnsLatency,
		&errorMsg,
	)
	if err != nil {
		return nil, fmt.Errorf("querying latest entry: %w", err)
	}

	if pingLatency.Valid {
		v := pingLatency.Float64
		entry.PingLatency = &v
	}
	if dnsLatency.Valid {
		v := dnsLatency.Float64
		entry.DNSLatency = &v
	}
	if errorMsg.Valid {
		v := errorMsg.String
		entry.ErrorMsg = &v
	}

	return &entry, nil
}

// GetRecentEntries returns the newest limit rows (newest first). limit is clamped to 1..500.
func (s *Store) GetRecentEntries(limit int) ([]ProbeLogEntry, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	query := `SELECT timestamp, status, rating, latency_ms, ping_latency_ms, dns_latency_ms, error_message
		FROM probe_logs ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent entries: %w", err)
	}
	defer rows.Close()

	var entries []ProbeLogEntry
	for rows.Next() {
		var entry ProbeLogEntry
		var pingLatency sql.NullFloat64
		var dnsLatency sql.NullFloat64
		var errorMsg sql.NullString

		if err := rows.Scan(
			&entry.Timestamp,
			&entry.Status,
			&entry.Rating,
			&entry.LatencyMs,
			&pingLatency,
			&dnsLatency,
			&errorMsg,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if pingLatency.Valid {
			v := pingLatency.Float64
			entry.PingLatency = &v
		}
		if dnsLatency.Valid {
			v := dnsLatency.Float64
			entry.DNSLatency = &v
		}
		if errorMsg.Valid {
			v := errorMsg.String
			entry.ErrorMsg = &v
		}

		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) GetEntriesSince(since time.Time) ([]ProbeLogEntry, error) {
	query := `SELECT timestamp, status, rating, latency_ms, ping_latency_ms, dns_latency_ms, error_message
		FROM probe_logs WHERE timestamp >= ? ORDER BY timestamp ASC`

	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("querying entries: %w", err)
	}
	defer rows.Close()

	var entries []ProbeLogEntry
	for rows.Next() {
		var entry ProbeLogEntry
		var pingLatency sql.NullFloat64
		var dnsLatency sql.NullFloat64
		var errorMsg sql.NullString

		if err := rows.Scan(
			&entry.Timestamp,
			&entry.Status,
			&entry.Rating,
			&entry.LatencyMs,
			&pingLatency,
			&dnsLatency,
			&errorMsg,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if pingLatency.Valid {
			v := pingLatency.Float64
			entry.PingLatency = &v
		}
		if dnsLatency.Valid {
			v := dnsLatency.Float64
			entry.DNSLatency = &v
		}
		if errorMsg.Valid {
			v := errorMsg.String
			entry.ErrorMsg = &v
		}

		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) GetStats(since time.Time) (*AggregatedStats, error) {
	// SUM(...) is NULL when no rows match; COUNT(*) is still 0 — COALESCE keeps Scan into int working.
	query := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END), 0) as up_count,
			COALESCE(SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END), 0) as down_count,
			AVG(CASE WHEN latency_ms > 0 THEN latency_ms END) as avg_latency,
			MIN(CASE WHEN latency_ms > 0 THEN latency_ms END) as min_latency,
			MAX(CASE WHEN latency_ms > 0 THEN latency_ms END) as max_latency,
			AVG(CAST(rating AS REAL)) as avg_rating
		FROM probe_logs
		WHERE timestamp >= ?
	`

	var stats AggregatedStats
	var avgLatency sql.NullFloat64
	var minLatency sql.NullFloat64
	var maxLatency sql.NullFloat64
	var avgRating sql.NullFloat64

	err := s.db.QueryRow(query, since).Scan(
		&stats.TotalProbes,
		&stats.UpProbes,
		&stats.DownProbes,
		&avgLatency,
		&minLatency,
		&maxLatency,
		&avgRating,
	)
	if err != nil {
		return nil, fmt.Errorf("querying stats: %w", err)
	}

	if stats.TotalProbes > 0 {
		stats.UptimePercent = float64(stats.UpProbes) / float64(stats.TotalProbes) * 100
	}
	if avgLatency.Valid {
		stats.AvgLatency = avgLatency.Float64
	}
	if minLatency.Valid {
		stats.MinLatency = minLatency.Float64
	}
	if maxLatency.Valid {
		stats.MaxLatency = maxLatency.Float64
	}
	if avgRating.Valid {
		stats.AvgRating = avgRating.Float64
	}

	return &stats, nil
}
