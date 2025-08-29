package stats

import (
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	stats := New()
	if stats.TotalCalls != 0 {
		t.Errorf("Expected 0 total calls, got %d", stats.TotalCalls)
	}

	stats.RecordCall(100*time.Millisecond, 10, 20)
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 total call, got %d", stats.TotalCalls)
	}

	statsMap := stats.Get()
	if statsMap["total_calls"] != 1 {
		t.Errorf("Expected 1 total call in map, got %d", statsMap["total_calls"])
	}
	if statsMap["avg_latency_ms"] != int64(100) {
		t.Errorf("Expected 100ms avg latency, got %d", statsMap["avg_latency_ms"])
	}
	if statsMap["total_chars_in"] != 10 {
		t.Errorf("Expected 10 total chars in, got %d", statsMap["total_chars_in"])
	}
	if statsMap["total_chars_out"] != 20 {
		t.Errorf("Expected 20 total chars out, got %d", statsMap["total_chars_out"])
	}
}
