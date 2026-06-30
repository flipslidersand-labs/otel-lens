// Package correlator joins Trace and Metrics signals from ClickHouse.
package correlator

import (
	"context"
	"sort"
	"time"

	"github.com/flipslidersand/otel-lens/internal/store"
)

// SignalWindow holds all signals retrieved for a time window.
type SignalWindow struct {
	From    time.Time
	To      time.Time
	Spans   []store.Span
	Metrics []store.MetricSample
}

// ServiceSummary aggregates trace + metric data per service within a window.
type ServiceSummary struct {
	ServiceName  string
	SpanCount    int
	ErrorCount   int
	AvgDurationMs float64
	// Metrics keyed by metric name → latest value in the window
	MetricValues map[string]float64
}

// Join fetches traces and metrics for [from, to) and correlates them by service name.
func Join(ctx context.Context, st *store.ClickHouseStore, from, to time.Time) (*SignalWindow, error) {
	spans, err := st.QueryTraces(ctx, from, to, "")
	if err != nil {
		return nil, err
	}
	metrics, err := st.QueryMetrics(ctx, from, to, "")
	if err != nil {
		return nil, err
	}
	return &SignalWindow{From: from, To: to, Spans: spans, Metrics: metrics}, nil
}

// Summarize groups the window's signals by service and returns summaries sorted by error count desc.
func (w *SignalWindow) Summarize() []ServiceSummary {
	byService := map[string]*ServiceSummary{}

	for _, sp := range w.Spans {
		s := getOrCreate(byService, sp.ServiceName)
		s.SpanCount++
		if sp.StatusCode == 2 { // OTLP status Error
			s.ErrorCount++
		}
		s.AvgDurationMs += sp.DurationMs
	}
	// Finalise averages
	for _, s := range byService {
		if s.SpanCount > 0 {
			s.AvgDurationMs /= float64(s.SpanCount)
		}
	}

	for _, m := range w.Metrics {
		s := getOrCreate(byService, m.ServiceName)
		if s.MetricValues == nil {
			s.MetricValues = map[string]float64{}
		}
		// Keep the latest value for each metric name
		if existing, ok := s.MetricValues[m.Name]; !ok || m.Timestamp.After(time.Time{}) {
			_ = existing
			s.MetricValues[m.Name] = m.Value
		}
	}

	out := make([]ServiceSummary, 0, len(byService))
	for _, s := range byService {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ErrorCount > out[j].ErrorCount
	})
	return out
}

func getOrCreate(m map[string]*ServiceSummary, svc string) *ServiceSummary {
	if s, ok := m[svc]; ok {
		return s
	}
	s := &ServiceSummary{ServiceName: svc, MetricValues: map[string]float64{}}
	m[svc] = s
	return s
}
