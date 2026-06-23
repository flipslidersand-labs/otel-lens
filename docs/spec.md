# Spec — OTelLens

## プロジェクトの目的

ログ・メトリクス・トレース・プロファイルを時刻と属性で関連付け、障害の原因候補をランキングする相関エンジン。
OTLP で受信し、ClickHouse に保存して、相関スコアで原因候補を算出する。

## 利用イメージ

```bash
# OTLP エンドポイントを起動
otellens serve --port 4317

# 相関クエリ
otellens correlate \
  --from "2026-06-22T10:00:00Z" \
  --to   "2026-06-22T10:05:00Z" \
  --signal "error_rate > 0.1"

# 出力
Correlation candidates:
  1. [0.92] DB connection pool exhausted (trace: abc123)
  2. [0.87] Deploy event: v2.3.1 at 09:58:00
  3. [0.71] CPU spike on node-3 (metric: cpu_usage)
```

## MVP の境界線

### やること (Phase 1〜4)

- OTLP gRPC で Trace / Metrics を受信
- ClickHouse にシグナルを保存
- Trace ID による Trace ↔ Metrics の関連付け
- 時系列でのエラー率集計・表示
- 指定時間帯の相関候補スコアリング

### やらないこと (Phase 1)

- Profiles シグナル
- デプロイイベント統合
- AI による原因説明
- 変化点検出 (changepoint detection)

## 成功条件

| Phase   | 完成条件                                            |
| ------- | --------------------------------------------------- |
| Phase 1 | OTLP gRPC で Trace を受信し ClickHouse に保存できる |
| Phase 2 | OTLP で Metrics を受信し保存できる                  |
| Phase 3 | Trace ID で Trace ↔ Metrics を関連付けられる        |
| Phase 4 | エラー率の時系列集計を CLI で表示できる             |
| Phase 5 | 相関スコアで原因候補をランキングできる              |
| Phase 6 | デプロイイベントを外部 Webhook で受信し相関に含める |
