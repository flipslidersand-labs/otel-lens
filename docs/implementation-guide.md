# Implementation Guide — OTelLens

## Phase 1: OTLP Trace 受信（1週）

- `internal/receiver/trace.go` — `go.opentelemetry.io/collector` の gRPC レシーバー
- 受信した Span を `chan Span` に投入
- `internal/store/clickhouse.go` — `traces` テーブルに INSERT

**完成条件**:

```bash
docker run -p 9000:9000 clickhouse/clickhouse-server
otellens serve
# OTel SDK からスパンを送信 → ClickHouse に保存される
```

---

## Phase 2: OTLP Metrics 受信（1週）

- `internal/receiver/metrics.go` — MetricsReceiver を実装
- `metrics` テーブルに INSERT

---

## Phase 3: Trace ↔ Metrics 相関（1週）

- `internal/correlator/join.go` — TraceID で Trace と Metrics を JOIN
- ClickHouse の `JOIN` クエリで同一時間帯のシグナルを取得

---

## Phase 4: エラー率時系列集計（3日）

- `otellens stats --from T1 --to T2` コマンド
- ClickHouse の `GROUP BY toStartOfMinute(start_time)` でエラー率を集計
- CLI でテーブル表示

---

## Phase 5: 相関スコアリング（1〜2週）

- `internal/correlator/score.go` — Pearson 相関係数の計算
- 異常シグナルとの相関が高いメトリクス・サービスをランキング
- `otellens correlate --signal "error_rate > 0.1" --window 5m`

---

## Phase 6: デプロイイベント統合（1週）

- `POST /events/deploy` で外部 Webhook を受信
- デプロイ時刻を `deploy_events` テーブルに保存
- 相関スコアにデプロイとの近接性を加味
