# Tech Stack — OTelLens

## 言語・バージョン

- Go 1.22+

## 主要パッケージ

| パッケージ                               | 役割                      | 選定理由                                    |
| ---------------------------------------- | ------------------------- | ------------------------------------------- |
| `go.opentelemetry.io/collector`          | OTLP 受信・パイプライン   | OTel Collector SDK で既存エコシステムと統合 |
| `github.com/ClickHouse/clickhouse-go/v2` | シグナル永続化            | 時系列・集計クエリが PostgreSQL より高速    |
| `google.golang.org/grpc`                 | OTLP gRPC エンドポイント  | OTel 標準トランスポート                     |
| `github.com/prometheus/client_golang`    | Prometheus メトリクス受信 | `/metrics` スクレイピング対応               |
| `github.com/spf13/cobra`                 | CLI                       | Go 標準 CLI フレームワーク                  |
| `go.uber.org/zap`                        | 構造化ログ                | 高速 JSON ログ                              |

## アーキテクチャ

```
OTel SDK (アプリ)
  ↓ OTLP gRPC :4317
[Receiver]
  ├── TraceReceiver   → chan Trace
  └── MetricsReceiver → chan Metrics
        ↓
[Store (ClickHouse)]
  ├── traces テーブル
  └── metrics テーブル
        ↓
[Correlator]
  ├── TraceID で Trace ↔ Metrics を JOIN
  ├── 時刻窓でシグナルをグループ化
  └── 相関スコア計算 (Pearson / cosine)
        ↓
[CLI Reporter]  原因候補ランキングを出力
```
