# ADR-001: シグナル保存に ClickHouse を使う

- **日付**: 2026-06-22
- **状態**: Accepted

## 決定

Trace / Metrics を ClickHouse に保存する。

## 理由

- 列指向 OLAP DB で時系列集計クエリが PostgreSQL より 10〜100 倍高速
- `toStartOfMinute()` / `GROUP BY` の組み合わせでエラー率集計がシンプルに書ける
- `clickhouse-go/v2` が context / batch insert に対応しており高スループット書き込みが容易
- OTel エコシステムで ClickHouse は標準的な保存先として採用例が多い

## トレードオフ

- `docker run clickhouse/clickhouse-server` が必要（組み込み不可）
- UPDATE / DELETE が苦手（ストリームデータの上書きには不向き）
