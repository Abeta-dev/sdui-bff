# AGENTS.md — AI Coding Agent Guide for `abeta-dev/sdui-bff`

This file provides routing, architectural invariants, and verification gates for autonomous coding agents working in **`github.com/abeta-dev/sdui-bff`** (Go `1.26.0` Clean Architecture Multi-Vertical SDUI Domain BFF).

---

## 1. Agent Skill & Code Owner Routing Table

Load the matching skill and Code Owner persona before planning or editing code:

| Task Domain | Skill / Guide Path | Persona / Code Owner Reference |
| :--- | :--- | :--- |
| Full SDUI BFF development (domain ports, verticals, Layout Engine client, usecases, Gin router, CI/CD) | [skills/abeta-sdui-bff/SKILL.md](skills/abeta-sdui-bff/SKILL.md) | [agents/bff-engineer.md](agents/bff-engineer.md) |
| Clean Architecture layers, Layout Engine hydration & KMP wire parity | [docs/architecture.md](docs/architecture.md) | [agents/bff-engineer.md](agents/bff-engineer.md) |
| 7 stateful vertical apps (`Fresh`, `Bites`, `Pulse`, `Mart`, `Spark`, `Trade`, `Pay`) & adding new verticals | [docs/verticals-guide.md](docs/verticals-guide.md) | [agents/bff-engineer.md](agents/bff-engineer.md) |

### Multi-IDE & Multi-Model Entrypoints
- **Claude Code**: [CLAUDE.md](CLAUDE.md)
- **Gemini / Antigravity / JetBrains AI**: [GEMINI.md](GEMINI.md)
- **GitHub Copilot & Codex**: [AGENTS.md](AGENTS.md)
- **Code Ownership**: [.github/CODEOWNERS](.github/CODEOWNERS)

---

## 2. Architectural Truth Gates (Non-Negotiable)

All changes MUST satisfy these invariants:

1. **Uncle Bob Dependency Rule (Strict Go Clean Architecture)**:
   - Layering flows strictly inward:
     `cmd/sdui-bff` -> `internal/handler` -> `internal/usecase` -> `internal/domain` <- `internal/infra` & `internal/verticals/*`.
   - `internal/domain` MUST remain pure Go with **zero** imports of `github.com/gin-gonic/gin`, `net/http`, `internal/handler`, `internal/infra`, `internal/usecase`, or `internal/verticals`.
   - This invariant is enforced automatically in CI by `internal/domain/arch_test.go`.
2. **Open/Closed (`OCP`) `VerticalModule` Pattern**:
   - Every vertical app (**Abeta Fresh**, **Abeta Bites**, **Abeta Pulse**, **Abeta Mart**, **Abeta Spark**, **Abeta Trade**, **Abeta Pay**) implements `domain.VerticalModule` (`Descriptor()`, `Screens()`, `CanHandleScreen(screenID)`, `RenderScreen(ctx, req)`) under `internal/verticals/<vertical>/`.
   - Never add monolithic `switch screenID` blocks inside `internal/usecase` or `internal/handler`. Adding a new vertical must require only implementing `domain.VerticalModule` and registering it in `cmd/sdui-bff`.
3. **1:1 `abeta-dev/kmp` Wire Protocol Consumption (`v0.6.0`)**:
   - Emit typed `v1` SDUI contracts directly from `github.com/abeta-dev/kmp/bff@v0.6.0` (`pkg/sdui/{actions,analytics,atoms,molecules,snippets,theming}` — **29 Wire Atoms** / **31 Compose UI Atoms** incl. `MediaPagerAtom` and the 12 extended production atoms `QrCode`, `OtpInput`, `Select`, `SegmentedControl`, `RadioGroup`, `Rating`, `Price`, `Badge`, `AlertBanner`, `StatTile`, `KeyValueRow`, `Chip`, **21 canonical SnippetTypes** including sticky `search_bar`, `swipe_deck`, `chip_group`, `nudge_bar`, `marquee_strip`, `order_tracker`, and server-molded `composed_block` with **36 `ComposedNodeKind`s** plus Express Quick-Commerce molecules from `pkg/sdui/molecules`, **18 ActionTypes**, **7 ambient `MotionLoopEffect`s**, and Free-Delivery threshold `FloatingCartSpec` docks).
   - Normalize every returned `PageResponse` in `internal/usecase/composer.go` via **`snippets.EnsureComponentIDs(&page)`** so 100% of screens carry deterministic, stable component IDs.
   - Ingest batched client analytics via `POST /api/v1/analytics/events` and expose live telemetry via `GET /api/v1/analytics/summary` backed by `internal/domain.AnalyticsStore`, `internal/usecase.AnalyticsService`, and thread-safe `internal/infra/analyticsstore.InMemoryStore` (100-event ring buffer).
   - Wrap all `/api/v1/*` JSON responses in `github.com/abeta-dev/go-libs@v0.4.0` `ginmw.Envelope` (`ginmw.RespondSuccess`, `ginmw.RespondError`, `ginmw.RespondErrorWithDetail`).
   - Return structured `follow_up_actions` (`[]actions.ActionTrigger`) on mutation responses so the KMP `RemoteApiCallExecutor` triggers celebrations, toasts, and navigation seamlessly.
4. **KMP String-Only Tracking Property Rule**:
   - In `abeta-dev/kmp`, `TrackingContract.properties` deserializes as `Map<String, String>` (`kotlinx.serialization`).
   - Every `TrackingContract` / analytics event property map emitted by any vertical or action response MUST contain **string values only** (`map[string]string` — never booleans, numbers, arrays, or nested objects inside tracking properties).
5. **Graceful Standalone & Layout Engine Dual-Mode**:
   - When `LAYOUT_ENGINE_URL` is unset, `internal/infra/layoutclient.Client` operates as a zero-overhead no-op (`Enabled() == false`) so `go run ./cmd/sdui-bff` starts cleanly with zero external dependencies.
   - When `LAYOUT_ENGINE_URL` is configured, screen composition queries `GET <baseURL>/api/v1/screens/<slug>` forwarding targeting headers (`X-Tenant-ID`, `X-Platform`, `X-City`, `X-User-Segment`, `X-Window-Class`) and automatically falls back to standalone vertical rendering on `404`, timeout, or network errors.
6. **Concurrency Safety & Race-Free State**:
   - All in-memory stateful stores across verticals (orders, bookings, swipe decks/matches, RFQ records) MUST guard concurrent access with `sync.RWMutex` (or atomic primitives) and pass `go test -count=1 -race ./...` cleanly.
7. **Security, CI & Brand/Path Hygiene**:
   - Never use third-party competitor brand names or machine-specific filesystem paths anywhere in code, docs, scripts, or workflows.
   - Use strictly Abeta showcase brands (`Abeta Fresh`, `Abeta Bites`, `Abeta Pulse`, `Abeta Mart`, `Abeta Spark`, `Abeta Trade`, `Abeta Pay`) and `abeta-dev` / `Abeta-dev`.
   - Always anchor `/sdui-bff` in `.gitignore` so `cmd/sdui-bff/` is never ignored.
   - Secret scanning in CI MUST use the MIT-licensed `gitleaks` CLI `v8.30.1` verified by SHA-256 (`551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb`) — never paid-gated GitHub Actions wrappers.
   - CI MUST aggregate all required checks behind the final **`CI Quality Gate`** (`ci-quality-gate`) job in `.github/workflows/ci.yml` (`if: always()`, failing if any upstream job did not succeed) for single-check branch protection.

---

## 3. Verification Commands

Run the checks relevant to your change — or the full gate before opening a PR:

```bash
# 1. Brand & machine-path hygiene
./scripts/check_brand_hygiene.sh

# 2. Formatting, static analysis, Clean Architecture guard & race-enabled tests
test -z "$(gofmt -l .)"
go vet ./...
go test -count=1 -race -cover ./...

# 3. Static binary build & smoke check
CGO_ENABLED=0 go build -o /tmp/abeta-sdui-bff-check ./cmd/sdui-bff && rm -f /tmp/abeta-sdui-bff-check

# 4. Complete verification gate (also available via `make verify`)
./scripts/verify_all.sh
```

> **Honesty Rule**: Never claim a build, lint, or test suite passed unless you executed it after your final edit and directly observed its passing output.
