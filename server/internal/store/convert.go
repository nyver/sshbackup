package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timeLayout is the text format used for every stored timestamp: UTC
// RFC3339 with nanosecond precision, so ordering within the same second
// stays well defined.
const timeLayout = time.RFC3339Nano

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}

func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*t), Valid: true}
}

func parseTimePtr(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func intToBool(v int64) bool {
	return v != 0
}

// joinStrings and splitStrings encode a small ordered list of strings
// (glob patterns, weekday numbers) as newline-separated text, which is
// simpler than a second table for data this small and never queried on.
func joinStrings(values []string) string {
	return strings.Join(values, "\n")
}

func splitStrings(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}

func splitInts(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	values := make([]int, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("parse stored int list %q: %w", s, err)
		}
		values[i] = v
	}
	return values, nil
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func intPtr(ns sql.NullInt64) *int {
	if !ns.Valid {
		return nil
	}
	v := int(ns.Int64)
	return &v
}
