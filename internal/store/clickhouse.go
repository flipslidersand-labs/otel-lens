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

// DeployEvent records a deployment for correlation scoring.
type DeployEvent struct {
	DeployedAt  time.Time
	ServiceName string
	Version     string
	Author      string
}

const ddlDeployEvents = `
CREATE TABLE IF NOT EXISTS deploy_events (
    deployed_at  DateTime64(9, 'UTC'),
    service_name String,
    version      String,
    author       String
) ENGINE = MergeTree()
  ORDER BY (service_name, deployed_at)
`

func (s *ClickHouseStore) CreateSchema(ctx context.Context) error {
	for _, ddl := range []string{ddlTraces, ddlMetrics, ddlDeployEvents} {
		if err := s.conn.Exec(ctx, ddl); err != nil {
			return err
		}
	}
	return nil
}

func (s *ClickHouseStore) InsertDeployEvent(ctx context.Context, e DeployEvent) error {
	return s.conn.Exec(ctx,
		`INSERT INTO deploy_events (deployed_at, service_name, version, author) VALUES (?,?,?,?)`,
		e.DeployedAt, e.ServiceName, e.Version, e.Author,
	)
}

// QueryDeployEvents returns deploy events in [from, to).
func (s *ClickHouseStore) QueryDeployEvents(ctx context.Context, from, to time.Time) ([]DeployEvent, error) {
	rows, err := s.conn.Query(ctx,
		`SELECT deployed_at, service_name, version, author
		 FROM deploy_events WHERE deployed_at >= ? AND deployed_at < ?
		 ORDER BY deployed_at`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []DeployEvent
	for rows.Next() {
		var e DeployEvent
		if err := rows.Scan(&e.DeployedAt, &e.ServiceName, &e.Version, &e.Author); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
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

// ErrorRateBucket holds aggregated error rate for one time bucket.
type ErrorRateBucket struct {
	Bucket       time.Time
	Total        uint64
	Errors       uint64
	ErrorRatePct float64 // 0–100
}

// QueryErrorRate returns per-minute error rate buckets in [from, to).
// service = "" means all services.
func (s *ClickHouseStore) QueryErrorRate(ctx context.Context, from, to time.Time, service string) ([]ErrorRateBucket, error) {
	q := `SELECT
	          toStartOfMinute(start_time)   AS bucket,
	          count()                        AS total,
	          countIf(status_code = 2)       AS errors
	      FROM traces
	      WHERE start_time >= ? AND start_time < ?`
	args := []any{from, to}
	if service != "" {
		q += " AND service_name = ?"
		args = append(args, service)
	}
	q += " GROUP BY bucket ORDER BY bucket"

	rows, err := s.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []ErrorRateBucket
	for rows.Next() {
		var b ErrorRateBucket
		if err := rows.Scan(&b.Bucket, &b.Total, &b.Errors); err != nil {
			return nil, err
		}
		if b.Total > 0 {
			b.ErrorRatePct = float64(b.Errors) / float64(b.Total) * 100
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

// MetricBucket is an averaged metric value for one minute bucket.
type MetricBucket struct {
	Bucket      time.Time
	MetricName  string
	ServiceName string
	AvgValue    float64
}

// QueryMetricTimeseries returns per-minute averaged values for each metric in [from, to).
func (s *ClickHouseStore) QueryMetricTimeseries(ctx context.Context, from, to time.Time) ([]MetricBucket, error) {
	q := `SELECT
	          toStartOfMinute(timestamp) AS bucket,
	          metric_name,
	          service_name,
	          avg(value) AS avg_value
	      FROM metrics
	      WHERE timestamp >= ? AND timestamp < ?
	      GROUP BY bucket, metric_name, service_name
	      ORDER BY bucket`

	rows, err := s.conn.Query(ctx, q, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []MetricBucket
	for rows.Next() {
		var b MetricBucket
		if err := rows.Scan(&b.Bucket, &b.MetricName, &b.ServiceName, &b.AvgValue); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

func (s *ClickHouseStore) Ping(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

func (s *ClickHouseStore) Close() error {
	return s.conn.Close()
}
