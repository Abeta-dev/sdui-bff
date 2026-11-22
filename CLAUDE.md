# CLAUDE.md — Claude Code Instructions for `abeta-dev/sdui-bff`

This repository is **Abeta SDUI Domain BFF (`github.com/abeta-dev/sdui-bff`)**, a standalone Go `1.26.0` Clean Architecture multi-vertical Server-Driven UI (SDUI) Backend-for-Frontend consuming `github.com/abeta-dev/kmp/bff@v0.3.0` and `github.com/abeta-dev/go-libs@v0.4.0`.

Read [AGENTS.md](AGENTS.md) for the complete architectural contract. Below is the operational quick reference for every session.

---

## Skill & Owner Routing

Before editing code, load:
- **Engineering Skill**: [skills/abeta-sdui-bff/SKILL.md](skills/abeta-sdui-bff/SKILL.md)
- **Code Owner Persona**: [agents/bff-engineer.md](agents/bff-engineer.md)
- **Deep Guides**: [docs/architecture.md](docs/architecture.md), [docs/verticals-guide.md](docs/verticals-guide.md)

---

## Architecture & Invariants

- **Clean Architecture Boundaries**:
  - `cmd/sdui-bff`: Entrypoint, env configuration (`PORT=8085`, `LAYOUT_ENGINE_URL`, `JWT_SECRET`, `RATE_LIMIT_RPS`), `VerticalModule` registration, HTTP/2 + `h2c` server, graceful shutdown.
  - `internal/domain`: Pure entities (`AppDescriptor`, `ScreenSummary`, `ScreenRequest`, `ActionExecuteRequest`, `ActionResult`, `ExpressOrderRequest`, `BookingRequest`, `SwipeRequest`, `SparkMatch`, `RFQRequest`, `RFQRecord`) and ports (`VerticalModule`, `LayoutEngineClient`, `ScreenComposer`, `ActionExecutor`). Zero HTTP/Gin/infra/vertical imports (guarded by `internal/domain/arch_test.go`).
  - `internal/usecase`: `ScreenComposer` (Layout Engine hydration + OCP `VerticalModule` dispatch) and `ActionExecutor`.
  - `internal/handler`: Gin router, vertical endpoints, Brotli/gzip compression, SHA-256 `ETag`, OpenAPI 3.1 (`/openapi.json`) & Scalar UI (`/docs`), `/healthz`, `/readyz`, `go-libs/ginmw` envelopes.
  - `internal/infra/layoutclient`: Optional HTTP client for `Abeta-dev/layout-engine` (5s timeout, 4 MiB response cap, targeting header forwarding, clean standalone no-op when `LAYOUT_ENGINE_URL` is unset).
  - `internal/verticals/*`: 6 OCP `VerticalModule` implementations (**Abeta Fresh**, **Abeta Bites**, **Abeta Pulse**, **Abeta Mart**, **Abeta Spark**, **Abeta Trade**) with mutex-protected stateful stores.
- **KMP String-Only Tracking Properties**: All `TrackingContract.properties` maps MUST use string values only (`map[string]string`) so Kotlin `kotlinx.serialization` decodes cleanly across all KMP targets.
- **Brand & Path Hygiene**: Strictly use Abeta showcase brands (`Abeta Fresh`, `Abeta Bites`, `Abeta Pulse`, `Abeta Mart`, `Abeta Spark`, `Abeta Trade`) and `abeta-dev` / `Abeta-dev`. Never commit third-party brand names or local filesystem paths.

---

## Essential Commands

```bash
# Run standalone on :8085 (zero external dependencies)
go run ./cmd/sdui-bff

# Run connected to a local Abeta-dev/layout-engine on :8090
LAYOUT_ENGINE_URL=http://localhost:8090 go run ./cmd/sdui-bff

# Brand & path hygiene check
./scripts/check_brand_hygiene.sh

# Format, vet, and race-enabled unit/integration tests
gofmt -w .
go vet ./...
go test -count=1 -race -cover ./...

# Full verification suite
./scripts/verify_all.sh
```
