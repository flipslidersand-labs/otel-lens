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

// QueryTraces returns spans in [from, to) for a service (empty = all services).
func (s *ClickHouseStore) QueryTraces(ctx context.Context, from, to time.Time, service string) ([]Span, error) {
	q := `SELECT trace_id, span_id, parent_span_id, service_name, operation,
	             start_time, end_time, duration_ms, status_code, attributes
	      FROM traces
	      WHERE start_time >= ? AND start_time < ?`
	args := []any{from, to}
	if service != "" {
		q += " AND service_name = ?"
		args = append(args, service)
	}
	rows, err := s.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spans []Span
	for rows.Next() {
		var sp Span
		if err := rows.Scan(&sp.TraceID, &sp.SpanID, &sp.ParentSpanID, &sp.ServiceName,
			&sp.Operation, &sp.StartTime, &sp.EndTime, &sp.DurationMs, &sp.StatusCode, &sp.Attributes); err != nil {
			return nil, err
		}
		spans = append(spans, sp)
	}
	return spans, rows.Err()
}

// QueryMetrics returns metric samples in [from, to) for a metric name (empty = all).
func (s *ClickHouseStore) QueryMetrics(ctx context.Context, from, to time.Time, metricName string) ([]MetricSample, error) {
	q := `SELECT metric_name, service_name, timestamp, value, labels
	      FROM metrics
	      WHERE timestamp >= ? AND timestamp < ?`
	args := []any{from, to}
	if metricName != "" {
		q += " AND metric_name = ?"
		args = append(args, metricName)
	}
	rows, err := s.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []MetricSample
	for rows.Next() {
		var m MetricSample
		if err := rows.Scan(&m.Name, &m.ServiceName, &m.Timestamp, &m.Value, &m.Labels); err != nil {
			return nil, err
		}
		samples = append(samples, m)
	}
	return samples, rows.Err()
}

func (s *ClickHouseStore) Ping(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

func (s *ClickHouseStore) Close() error {
	return s.conn.Close()
}
