# otel-lens

OpenTelemetry signal correlation engine — connects distributed traces, metrics, and logs for automated root-cause diagnosis (Go + ClickHouse).

> 🟡 **Scaffold Phase** — Design in progress, MVP implementation starting Q3 2026

## Status

| Item | State |
|------|------|
| Phase | Scaffold |
| MVP Target | Q3 2026 |
| Tests | Planned |
| Docs | Planned |

## Planned Tech Stack

- **Language:** Go
- **Signals:** OpenTelemetry traces / metrics / logs
- **Backend:** ClickHouse

## Roadmap

1. **Phase 1 (Q3 2026)** — OTLP ingestion + storage layer
2. **Phase 2 (Q4 2026)** — Cross-signal correlation engine
3. **Phase 3 (Q1 2027)** — Root-cause scoring + alerting
4. **Phase 4 (Q2 2027)** — Dashboard + production deployment

## Notes

- API and design subject to change during scaffold phase
- No test coverage yet — quality guarantees begin at Phase 2

