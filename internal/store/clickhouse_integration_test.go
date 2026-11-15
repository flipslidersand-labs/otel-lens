package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// Requires a ClickHouse server: set OTEL_LENS_TEST_CLICKHOUSE=host:port
// (e.g. docker run -p 127.0.0.1:19000:9000 -e CLICKHOUSE_SKIP_USER_SETUP=1 clickhouse/clickhouse-server:25.8).
func newTestStore(t *testing.T) *ClickHouseStore {
	t.Helper()
	addr := os.Getenv("OTEL_LENS_TEST_CLICKHOUSE")
	if addr == "" {
		t.Skip("OTEL_LENS_TEST_CLICKHOUSE not set")
	}
	s, err := New(addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.CreateSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSubSecondPrecisionPreserved(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2030, 1, 2, 10, 2, 33, 0, time.UTC)
	end := base.Add(120 * time.Millisecond)
	svc := "subsec-" + time.Now().Format("150405.000000")

	if err := s.InsertSpan(ctx, Span{TraceID: "t", SpanID: "s", ServiceName: svc, Operation: "op",
		StartTime: base, EndTime: end, DurationMs: 120, Attributes: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	spans, err := s.QueryTraces(ctx, base.Add(-time.Second), base.Add(time.Second), svc)
	if err != nil || len(spans) != 1 {
		t.Fatalf("spans=%d err=%v", len(spans), err)
	}
	if d := spans[0].EndTime.Sub(spans[0].StartTime); d != 120*time.Millisecond {
		t.Errorf("trace end-start = %v, want 120ms", d)
	}

	ts := base.Add(456 * time.Millisecond)
	if err := s.InsertMetric(ctx, MetricSample{Name: svc, ServiceName: svc, Timestamp: ts, Value: 1, Labels: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	ms, err := s.QueryMetrics(ctx, base.Add(-time.Second), base.Add(time.Second), svc)
	if err != nil || len(ms) != 1 {
		t.Fatalf("metrics=%d err=%v", len(ms), err)
	}
	if !ms[0].Timestamp.Equal(ts) {
		t.Errorf("metric ts = %v, want %v", ms[0].Timestamp, ts)
	}

	dt := base.Add(789 * time.Millisecond)
	if err := s.InsertDeployEvent(ctx, DeployEvent{DeployedAt: dt, ServiceName: svc, Version: "v", Author: "a"}); err != nil {
		t.Fatal(err)
	}
	evs, err := s.QueryDeployEvents(ctx, base.Add(-time.Second), base.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range evs {
		if e.ServiceName == svc {
			found = true
			if !e.DeployedAt.Equal(dt) {
				t.Errorf("deployed_at = %v, want %v", e.DeployedAt, dt)
			}
		}
	}
	if !found {
		t.Error("deploy event not found")
	}
}
