# Data Model — OTelLens

## ClickHouse スキーマ

```sql
-- Trace スパン
CREATE TABLE traces (
    trace_id      String,
    span_id       String,
    parent_span_id String,
    service_name  String,
    operation     String,
    start_time    DateTime64(9, 'UTC'),
    end_time      DateTime64(9, 'UTC'),
    duration_ms   Float64,
    status_code   UInt8,    -- 0: Unset, 1: Ok, 2: Error
    attributes    Map(String, String)
) ENGINE = MergeTree()
  ORDER BY (service_name, start_time);

-- メトリクスサンプル
CREATE TABLE metrics (
    metric_name   String,
    service_name  String,
    timestamp     DateTime64(9, 'UTC'),
    value         Float64,
    labels        Map(String, String)
) ENGINE = MergeTree()
  ORDER BY (metric_name, service_name, timestamp);
```

## Go 構造体

```go
type Span struct {
    TraceID     string
    SpanID      string
    ServiceName string
    Operation   string
    StartTime   time.Time
    Duration    time.Duration
    StatusCode  int
    Attributes  map[string]string
}

type MetricSample struct {
    Name        string
    ServiceName string
    Timestamp   time.Time
    Value       float64
    Labels      map[string]string
}

// 相関候補
type Correlation struct {
    Description string
    Score       float64  // 0.0〜1.0
    Evidence    []string // 関連するシグナルの説明
    TraceIDs    []string
}
```

## 相関スコアリング

```
入力: 時間窓 [T1, T2]、異常シグナル (error_rate > 0.1)

1. ClickHouse で [T1-5m, T2] のすべてのシグナルを取得
2. 異常シグナルと各メトリクスの Pearson 相関係数を計算
3. Trace の error span を集計しサービス別にランキング
4. 相関係数 × ログ出現頻度でスコアを算出
5. 上位 N 件を Correlation として返す
```
