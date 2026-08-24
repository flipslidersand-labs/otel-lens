# otel-lens

OpenTelemetry signal correlation engine for distributed tracing analysis and root-cause diagnosis (Go + ClickHouse).

分散トレーシング分析・根本原因診断のための OpenTelemetry シグナル相関エンジン。

> 🟡 **Scaffold Phase** — 基本設計と MVP 実装準備中

## Status / ステータス

| Item | State | 状態 |
|------|------|------|
| Phase / フェーズ | Scaffold | Scaffold |
| MVP Start / 実装予定 | Q3 2026 | 2026-Q3 MVP開始 |
| Tests / テスト | Not yet | 未実装 |
| Docs / ドキュメント | Planned | 企画中 |

## Tech Stack / 技術スタック

- **Language / 言語:** Go
- **Signals / シグナル:** OpenTelemetry traces / metrics / logs
- **Backend:** ClickHouse

## Roadmap / 実装ロードマップ

1. **Phase 1 (Q3 2026)** — OTLP ingestion + storage / OTLP 取込み + ストレージ
2. **Phase 2 (Q4 2026)** — Cross-signal correlation engine / クロスシグナル相関エンジン
3. **Phase 3 (Q1 2027)** — Root-cause scoring + alerting / 根本原因スコアリング + アラート
4. **Phase 4 (Q2 2027)** — Dashboard + production deployment / ダッシュボード + 本番デプロイ

## Notes / 注意事項

- **In Development / 開発中**: API/design may change without notice / API・設計は予告なく変更される可能性があります
- **No tests yet / テスト未実装**: Quality not guaranteed until Phase 2 / 本格実装まで品質保証していません

---

Track progress in [GitHub Issues](https://github.com/flipslidersand/otel-lens/issues).
