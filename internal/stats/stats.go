package stats

import (
	"log"
	"sync"
	"time"
)

type Stats struct {
	mu            sync.Mutex
	TotalCalls    int           `json:"total_calls"`
	TotalLatency  time.Duration `json:"total_latency"`
	TotalCharsIn  int           `json:"total_chars_in"`
	TotalCharsOut int           `json:"total_chars_out"`
}

func New() *Stats {
	return &Stats{}
}

func (s *Stats) RecordCall(latency time.Duration, charsIn, charsOut int) {
	log.Printf("Recording call: latency=%v, charsIn=%d, charsOut=%d\n", latency, charsIn, charsOut)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalCalls++
	s.TotalLatency += latency
	s.TotalCharsIn += charsIn
	s.TotalCharsOut += charsOut
}

func (s *Stats) Get() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	avgLatency := int64(0)
	if s.TotalCalls > 0 {
		avgLatency = s.TotalLatency.Milliseconds() / int64(s.TotalCalls)
	}
	return map[string]interface{}{
		"total_calls":     s.TotalCalls,
		"avg_latency_ms":  avgLatency,
		"total_chars_in":  s.TotalCharsIn,
		"total_chars_out": s.TotalCharsOut,
	}
}
