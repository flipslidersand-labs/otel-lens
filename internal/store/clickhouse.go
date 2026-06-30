package store

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Span struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	ServiceName  string
	Operation    string
	StartTime    time.Time
	EndTime      time.Time
	DurationMs   float64
	StatusCode   uint8
	Attributes   map[string]string
}

type ClickHouseStore struct {
	conn driver.Conn
}

func New(addr string) (*ClickHouseStore, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:  []string{addr},
		Debug: false,
	})
	if err != nil {
		return nil, err
	}
	return &ClickHouseStore{conn: conn}, nil
}

const ddlTraces = `
CREATE TABLE IF NOT EXISTS traces (
    trace_id       String,
    span_id        String,
    parent_span_id String,
    service_name   String,
    operation      String,
    start_time     DateTime64(9, 'UTC'),
    end_time       DateTime64(9, 'UTC'),
    duration_ms    Float64,
    status_code    UInt8,
    attributes     Map(String, String)
) ENGINE = MergeTree()
  ORDER BY (service_name, start_time)
`

func (s *ClickHouseStore) InsertSpan(ctx context.Context, span Span) error {
	return s.conn.Exec(ctx,
		`INSERT INTO traces
		 (trace_id, span_id, parent_span_id, service_name, operation,
		  start_time, end_time, duration_ms, status_code, attributes)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		span.TraceID, span.SpanID, span.ParentSpanID,
		span.ServiceName, span.Operation,
		span.StartTime, span.EndTime,
		span.DurationMs, span.StatusCode, span.Attributes,
	)
}

type MetricSample struct {
	Name        string
	ServiceName string
	Timestamp   time.Time
	Value       float64
	Labels      map[string]string
}

const ddlMetrics = `
CREATE TABLE IF NOT EXISTS metrics (
    metric_name  String,
    service_name String,
    timestamp    DateTime64(9, 'UTC'),
    value        Float64,
    labels       Map(String, String)
) ENGINE = MergeTree()
  ORDER BY (metric_name, service_name, timestamp)
`

func (s *ClickHouseStore) CreateSchema(ctx context.Context) error {
	if err := s.conn.Exec(ctx, ddlTraces); err != nil {
		return err
	}
	return s.conn.Exec(ctx, ddlMetrics)
}

func (s *ClickHouseStore) InsertMetric(ctx context.Context, m MetricSample) error {
	return s.conn.Exec(ctx,
		`INSERT INTO metrics (metric_name, service_name, timestamp, value, labels)
		 VALUES (?,?,?,?,?)`,
		m.Name, m.ServiceName, m.Timestamp, m.Value, m.Labels,
	)
}

func (s *ClickHouseStore) Ping(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

func (s *ClickHouseStore) Close() error {
	return s.conn.Close()
}
