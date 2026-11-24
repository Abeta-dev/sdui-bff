---
name: abeta-sdui-bff
description: Develop, test, and operate the Abeta Multi-Vertical SDUI Domain BFF (github.com/abeta-dev/sdui-bff v0.4.0, Go 1.26.0 Clean Architecture) powering 7 stateful vertical apps (Abeta Fresh, Abeta Bites, Abeta Pulse, Abeta Mart, Abeta Spark, Abeta Trade, Abeta Pay) over github.com/abeta-dev/kmp/bff@v0.6.0 and github.com/abeta-dev/go-libs@v0.4.0 with optional Abeta-dev/layout-engine hydration.
---

# Abeta SDUI Domain BFF (`abeta-dev/sdui-bff`)

Use this skill whenever designing, implementing, debugging, or reviewing features in `github.com/abeta-dev/sdui-bff`.

---

## 1. Clean Architecture Package Layout

```text
sdui-bff/
├── cmd/sdui-bff/              # Entrypoint, env config, OCP VerticalModule wiring, HTTP/2 & h2c server, graceful shutdown
├── internal/
│   ├── domain/                # Pure entities, sentinel errors, ports (VerticalModule, LayoutEngineClient, ScreenComposer, ActionExecutor, AnalyticsStore, AnalyticsService) + arch_test.go
│   ├── usecase/               # ScreenComposer (EnsureComponentIDs + Layout Engine + Fallback), ActionExecutor, AnalyticsService
│   ├── handler/               # Gin router, vertical & analytics endpoints, OpenAPI 3.1 (/openapi.json, /docs), Brotli/Gzip, ETag, health probes
│   ├── infra/
│   │   ├── analyticsstore/    # Thread-safe in-memory AnalyticsStore (counters + 100-event ring buffer)
│   │   └── layoutclient/      # Optional HTTP client for Abeta-dev/layout-engine (5s timeout, 4 MiB cap, targeting headers)
│   └── verticals/             # 7 OCP VerticalModule packages: fresh (Express molecules), bites, pulse, mart, spark, trade, pay
├── scripts/                   # check_brand_hygiene.sh, verify_all.sh
├── docs/                      # architecture.md, verticals-guide.md
├── agents/                    # bff-engineer.md Code Owner persona
└── .github/workflows/         # ci.yml (hygiene + gitleaks CLI, race tests, static build, Docker build + CI Quality Gate), release.yml
```

### Dependency Rule Enforcement
- `internal/domain` MUST NEVER import `github.com/gin-gonic/gin`, `net/http`, `internal/handler`, `internal/infra`, `internal/usecase`, or `internal/verticals`.
- `internal/domain/arch_test.go` automatically fails `go test ./...` if any forbidden import is introduced.

---

## 2. Wire Contract & KMP Parity Rules

1. **Reuse `abeta-dev/kmp/bff@v0.6.0` Contracts Directly**:
   - Import `github.com/abeta-dev/kmp/bff/pkg/sdui/{actions,analytics,atoms,molecules,snippets,theming}` directly — never duplicate wire structs.
   - Maintain full parity with all **29 Wire Atoms** / **31 Compose UI Atoms** (incl. `CountdownAtom`, `MediaPagerAtom`, & the 12 extended atoms `QrCode`, `OtpInput`, `Select`, `SegmentedControl`, `RadioGroup`, `Rating`, `Price`, `Badge`, `AlertBanner`, `StatTile`, `KeyValueRow`, `Chip`), **21 canonical SnippetTypes** (including sticky `search_bar`, `swipe_deck`, `chip_group`, `product_card`, `nudge_bar`, `marquee_strip`, `order_tracker`, and server-molded `composed_block` with **36 `ComposedNodeKind`s** plus Express Quick-Commerce `molecules`), deterministic `snippets.EnsureComponentIDs`, batched analytics (`POST /api/v1/analytics/events`, `GET /api/v1/analytics/summary`), **18 ActionTypes**, **7 ambient `MotionLoopEffect`s**, and Free-Delivery threshold `FloatingCartSpec` docks.
2. **Envelope Compatibility**:
   - Wrap all `/api/v1/*` JSON responses in `github.com/abeta-dev/go-libs@v0.4.0` `ginmw.Envelope` (`ginmw.RespondSuccess`, `ginmw.RespondError`, `ginmw.RespondErrorWithDetail`).
3. **KMP String-Only Tracking Property Rule**:
   - `TrackingContract.properties` on `abeta-dev/kmp` deserializes strictly as `Map<String, String>`.
   - Always emit string-only values inside every tracking property map (never ints, bools, floats, or nested maps).
4. **Structured `follow_up_actions`**:
   - Mutation endpoints (`/api/v1/fresh/orders/express`, `/api/v1/bites/orders`, `/api/v1/pulse/bookings`, `/api/v1/mart/orders`, `/api/v1/spark/swipe`, `/api/v1/trade/rfq`, `/api/v1/pay/transfers`, `/api/v1/pay/cards/controls`, `/api/v1/orders/express`, `/api/v1/actions/execute`) must return `domain.ActionResult` with typed `actions.ActionTrigger` entries (`trigger_feedback`, `show_toast`, `navigate`) so KMP's `RemoteApiCallExecutor` triggers UX feedback automatically.

---

## 3. OCP `VerticalModule` & Dual-Mode Rules

- **Open/Closed Vertical Registration**:
  - Implement `domain.VerticalModule` (`var _ domain.VerticalModule = (*Module)(nil)`) inside `internal/verticals/<vertical>/`.
  - Keep vertical state concurrency-safe with `sync.RWMutex`.
- **Dual-Mode Layout Engine Integration**:
  - When `LAYOUT_ENGINE_URL` is unset, `sdui-bff` starts cleanly on `:8085` with zero external dependencies.
  - When `LAYOUT_ENGINE_URL` is set (e.g., `http://localhost:8090` or `http://layout-engine:8090` in Docker Compose), `layoutclient.Client` forwards targeting headers (`X-Tenant-ID`, `X-Platform`, `X-City`, `X-User-Segment`, `X-Window-Class`) and falls back gracefully to standalone vertical rendering on any error or `404`.
- **Brand, Path & Gitignore Hygiene**:
  - Strictly use Abeta showcase brands (`Abeta Fresh`, `Abeta Bites`, `Abeta Pulse`, `Abeta Mart`, `Abeta Spark`, `Abeta Trade`, `Abeta Pay`). Never introduce third-party brand names or local filesystem paths.
  - Keep `/sdui-bff` anchored in `.gitignore` so `cmd/sdui-bff/` is never ignored.

---

## 4. Verification Commands

```bash
# Brand & machine-path hygiene
./scripts/check_brand_hygiene.sh

# Format, static analysis, race tests & binary compilation
test -z "$(gofmt -l .)"
go vet ./...
go test -count=1 -race -cover ./...
CGO_ENABLED=0 go build -o /tmp/abeta-sdui-bff-check ./cmd/sdui-bff && rm -f /tmp/abeta-sdui-bff-check

# Full verification pipeline (or `make verify`)
./scripts/verify_all.sh
```
