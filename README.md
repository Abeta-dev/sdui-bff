# Abeta SDUI Domain BFF (`abeta-dev/sdui-bff`)

[![CI](https://github.com/Abeta-dev/sdui-bff/actions/workflows/ci.yml/badge.svg)](https://github.com/Abeta-dev/sdui-bff/actions/workflows/ci.yml)
[![Release](https://github.com/Abeta-dev/sdui-bff/actions/workflows/release.yml/badge.svg)](https://github.com/Abeta-dev/sdui-bff/actions/workflows/release.yml)
[![Go 1.26.0](https://img.shields.io/badge/Go-1.26.0-00ADD8.svg?logo=go)](https://go.dev/)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**Abeta SDUI Domain BFF** (`github.com/abeta-dev/sdui-bff`) is a standalone **Go 1.26.0** Clean Architecture **Backend-for-Frontend (BFF)** powering **6 stateful Super-App vertical experiences** over Server-Driven UI (SDUI) wire contracts.

It consumes typed `v1` SDUI contracts directly from [`github.com/abeta-dev/kmp/bff@v0.2.0`](https://github.com/Abeta-dev/kmp) (**15 Wire Atoms**, **17 SnippetTypes** including `swipe_deck`, `search_bar`, and `chip_group`, and **18 ActionTypes**) wrapped in [`github.com/abeta-dev/go-libs@v0.4.0`](https://github.com/Abeta-dev/go-libs) `ginmw.Envelope` responses, and optionally integrates with [`Abeta-dev/layout-engine`](https://github.com/Abeta-dev/layout-engine) for dynamic CMS-managed layout hydration with automatic standalone fallback.

---

## The 3-Repo Abeta SDUI Platform

| Repository | Role | Default Port / Runtime |
| :--- | :--- | :--- |
| **[`Abeta-dev/kmp`](https://github.com/Abeta-dev/kmp)** | Kotlin Multiplatform + Compose Multiplatform SDUI Client Kit (`v0.2.0` contracts, 16 Compose atoms, 17 snippet renderers, 18 OCP actions, `:sample-app` 6-in-1 Super-App Showcase, `:storybook`, `:watch-companion`) | Android · iOS · macOS · Desktop JVM · WasmJS · watchOS · Wear OS |
| **[`Abeta-dev/layout-engine`](https://github.com/Abeta-dev/layout-engine)** | Multi-tenant Dynamic SDUI Layout Engine & Visual Layout Studio (rule targeting, UTC scheduling, dynamic slotting, immutable snapshots, rollback, fallback) | `:8090` (SQLite WAL or PostgreSQL 17) |
| **[`Abeta-dev/sdui-bff`](https://github.com/Abeta-dev/sdui-bff)** *(this repo)* | Stateful Multi-Vertical Domain BFF orchestrating domain state, search/filtering, interactive mutations (`follow_up_actions`), and optional Layout Engine screen composition | `:8085` (zero-dependency standalone or integrated via `LAYOUT_ENGINE_URL`) |

---

## Architecture Overview

Dependencies follow the **Uncle Bob Dependency Rule** strictly inward (`internal/domain` has zero Gin, `net/http`, handler, usecase, vertical, or infra imports — enforced in CI by `internal/domain/arch_test.go`).

```mermaid
flowchart TB
    subgraph Clients["Multiplatform Clients (Abeta-dev/kmp)"]
        KMP["Android · iOS · macOS · Desktop JVM · WasmJS · Watch\n--bff-url=http://localhost:8085"]
    end

    subgraph BFF["Abeta SDUI Domain BFF — Go 1.26.0 (:8085)"]
        CMD["cmd/sdui-bff\nEnv Config · OCP Vertical Registration · HTTP/2 & h2c · Graceful Shutdown"]
        HANDLER["internal/handler\nGin Router · Brotli/Gzip · SHA-256 ETag · OpenAPI 3.1 (/openapi.json, /docs)\nOptional JWT & Rate Limit · go-libs/ginmw Envelopes"]
        USECASE["internal/usecase\nScreenComposer (Layout Engine Hydration + Vertical Fallback) · ActionExecutor"]
        DOMAIN["internal/domain\nPure Models, Sentinel Errors & Ports (VerticalModule, LayoutEngineClient)\nZero HTTP/Gin/Infra imports — enforced by arch_test.go"]
        VERTICALS["internal/verticals/*\n6 OCP VerticalModule Implementations\nFresh · Bites · Pulse · Mart · Spark · Trade"]
        LAYOUT_CLIENT["internal/infra/layoutclient\nOptional HTTP Client (5s timeout, 4 MiB cap, targeting headers)"]

        CMD --> HANDLER
        CMD --> USECASE
        CMD --> VERTICALS
        CMD --> LAYOUT_CLIENT
        HANDLER --> USECASE
        USECASE --> DOMAIN
        VERTICALS -. "implements domain.VerticalModule" .-> DOMAIN
        LAYOUT_CLIENT -. "implements domain.LayoutEngineClient" .-> DOMAIN
    end

    subgraph CMS["Optional Dynamic CMS Layer"]
        LE["Abeta-dev/layout-engine (:8090)\nRule Targeting · Slotting · Visual Studio (/studio)"]
        PG[("PostgreSQL 17\n(or SQLite WAL)")]
        LE --> PG
    end

    KMP <-- "ginmw.Envelope + v1 PageResponse\n(17 Snippets · 18 Actions · follow_up_actions)" --> HANDLER
    LAYOUT_CLIENT -. "GET /api/v1/screens/:slug\n(when LAYOUT_ENGINE_URL is set)" .-> LE
```

---

## 6 Stateful Super-App Verticals

Each vertical is registered as an Open/Closed (`OCP`) `domain.VerticalModule` with live filtering, search, state transitions, and rich `follow_up_actions` (`trigger_feedback` celebrations, toasts, and navigation).

| Vertical App | Category | Tenant ID | Theme Preset | Screen Routes (`GET /api/v1/screens/:id`) | Domain Mutation Endpoints |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Abeta Fresh** | Quick-Commerce (8-Min Grocery) | `abeta-fresh` | `EMERALD` | `fresh-home` | `POST /api/v1/fresh/orders/express`, `POST /api/v1/orders/express` |
| **Abeta Bites** | Food Delivery & Dining | `abeta-bites` | `AMBER` | `bites-home`, `bites-restaurant`, `bites-order-track` | `POST /api/v1/bites/orders` |
| **Abeta Pulse** | Live Events, Movies & Experiences | `abeta-pulse` | `INDIGO` | `pulse-home`, `pulse-event-detail` | `POST /api/v1/pulse/bookings` |
| **Abeta Mart** | Multi-Category Marketplace | `abeta-mart` | `AMBER` | `mart-home`, `mart-product-detail` | `POST /api/v1/mart/orders` |
| **Abeta Spark** | Swipe Discovery & Matchmaking | `abeta-spark` | `INDIGO` | `spark-deck`, `spark-matches` | `POST /api/v1/spark/swipe` |
| **Abeta Trade** | B2B Wholesale & RFQ Procurement | `abeta-trade` | `EMERALD` | `trade-home`, `trade-supplier` | `POST /api/v1/trade/rfq` |

### Core Discovery, Action & Ops Endpoints
- `GET /api/v1/apps` — Super-App catalog describing all 6 vertical apps
- `GET /api/v1/screens` — List all discoverable screen routes across verticals
- `GET /api/v1/screens/:screenId` — Fetch a `v1` `PageResponse` (supports `?q=`, `?filter=`, `?city=`, `?cursor=`, and `X-Tenant-ID`, `X-Platform`, `X-City`, `X-User-Segment`, `X-Window-Class`)
- `POST /api/v1/actions/execute` — Execute generic SDUI actions and return `follow_up_actions`
- `GET /openapi.json` & `GET /docs` — Live OpenAPI 3.1 schema and interactive Scalar API documentation
- `GET /healthz` & `GET /readyz` — Liveness and readiness probes

---

## Quick Start

### Mode 1: Standalone (Zero External Dependencies)

Requires only **Go 1.26.0** — no Docker, PostgreSQL, or Layout Engine needed:

```bash
git clone https://github.com/Abeta-dev/sdui-bff.git
cd sdui-bff
go run ./cmd/sdui-bff
```

The BFF starts on **`:8085`** with all 6 stateful verticals ready immediately:

```bash
curl http://localhost:8085/healthz
curl http://localhost:8085/api/v1/apps
curl "http://localhost:8085/api/v1/screens/fresh-home?city=Bengaluru"
```

### Mode 2: Connected to Local `Abeta-dev/layout-engine`

Start `Abeta-dev/layout-engine` on `:8090`, then run `sdui-bff` pointing to it:

```bash
LAYOUT_ENGINE_URL=http://localhost:8090 go run ./cmd/sdui-bff
# or via Makefile:
make run-with-layout-engine
```

When `LAYOUT_ENGINE_URL` is set, `sdui-bff` queries the Layout Engine first (forwarding `X-Tenant-ID`, `X-Platform`, `X-City`, `X-User-Segment`, and `X-Window-Class`) and falls back gracefully to standalone vertical rendering on `404`, timeout, or network error.

### Mode 3: Full 3-Service Platform Stack (Docker Compose)

Launch **PostgreSQL 17** + **`Abeta-dev/layout-engine` (`:8090`)** + **`Abeta-dev/sdui-bff` (`:8085`)** together:

```bash
docker compose up --build
```

- **SDUI BFF API**: [http://localhost:8085/docs](http://localhost:8085/docs)
- **Layout Engine Visual Studio**: [http://localhost:8090/studio](http://localhost:8090/studio)

---

## Connecting `Abeta-dev/kmp` (`sample-app`)

Launch the `Abeta-dev/kmp` 6-in-1 Super-App Showcase pointed directly at `sdui-bff`:

```bash
(cd ../kmp/kmp && ./gradlew :sample-app:run --args="--bff-url=http://localhost:8085")
```

Run headless end-to-end verification across all 6 verticals:

```bash
(cd ../kmp/kmp && ./gradlew :sample-app:run --args="--headless --bff-url=http://localhost:8085")
```

---

## Configuration

Copy [.env.example](.env.example) to `.env` for local overrides:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8085` | HTTP server listen port |
| `GIN_MODE` | `release` | Gin mode (`release`, `debug`, `test`) |
| `LAYOUT_ENGINE_URL` | *(empty)* | Optional `Abeta-dev/layout-engine` base URL (e.g. `http://localhost:8090`) |
| `LOG_LEVEL` | `info` | Structured log verbosity (`debug`, `info`, `warn`, `error`) |
| `JWT_SECRET` | *(empty)* | Optional HS256 JWT secret enabling bearer auth middleware when set |
| `RATE_LIMIT_RPS` | `0` | Optional global rate limit (requests/sec); `0` disables throttling |

---

## Development & Verification

```bash
make run                     # Start standalone on :8085
make run-with-layout-engine  # Start on :8085 wired to http://localhost:8090
make test                    # Run go test -count=1 -race -cover ./...
make vet                     # Run go vet ./...
make fmt                     # Format Go sources with gofmt
make verify                  # Run full ./scripts/verify_all.sh pipeline
make docker-build            # Build local distroless OCI image
make docker-up               # Start full 3-service Compose stack
```

---

## Documentation & AI Agent Guides

- [docs/architecture.md](docs/architecture.md) — Clean Architecture layers, `VerticalModule` OCP design, Layout Engine hydration, and KMP wire invariants
- [docs/verticals-guide.md](docs/verticals-guide.md) — Deep guide to all 6 stateful vertical apps, endpoints, interactive snippets, and adding new verticals
- [AGENTS.md](AGENTS.md) · [CLAUDE.md](CLAUDE.md) · [GEMINI.md](GEMINI.md) — Multi-IDE AI coding agent contracts and verification gates
- [skills/abeta-sdui-bff/SKILL.md](skills/abeta-sdui-bff/SKILL.md) · [agents/bff-engineer.md](agents/bff-engineer.md) — Domain engineering skill and Code Owner persona
- [LICENSE](LICENSE) — Apache License 2.0
