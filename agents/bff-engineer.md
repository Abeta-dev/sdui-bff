---
name: bff-engineer
module: "sdui-bff"
layer: "Go 1.26.0 Clean Architecture Multi-Vertical SDUI Domain BFF"
owned_paths:
  - "cmd/**"
  - "internal/**"
  - "docs/**"
  - "scripts/**"
  - "skills/**"
  - "agents/**"
  - ".github/**"
  - "Dockerfile"
  - "docker-compose.yaml"
  - "Makefile"
allowed_dependencies:
  - "cmd/sdui-bff -> internal/handler -> internal/usecase -> internal/domain <- internal/infra & internal/verticals/*"
  - "github.com/abeta-dev/kmp/bff@v0.6.0 (pkg/sdui/{actions,analytics,atoms,molecules,snippets,theming})"
  - "github.com/abeta-dev/go-libs@v0.4.0 (ginmw, apperror, health, logger, env, shutdown)"
  - "github.com/gin-gonic/gin, github.com/andybalholm/brotli"
forbidden_dependencies:
  - "internal/domain importing gin, net/http, internal/handler, internal/infra, internal/usecase, or internal/verticals (enforced by internal/domain/arch_test.go)"
  - "Non-string values in TrackingContract.properties maps"
  - "Hardcoded secrets, third-party competitor brand names, or machine-specific absolute paths"
required_skills:
  - "skills/abeta-sdui-bff/SKILL.md"
verification_command: "./scripts/verify_all.sh"
---

# Code Owner Agent: Abeta SDUI Domain BFF Engineer (`bff-engineer`)

## Ownership Scope
- **Repository**: `github.com/abeta-dev/sdui-bff` (`Abeta-dev/sdui-bff`, Apache-2.0)
- **Core Responsibilities**:
  - Go `1.26.0` Clean Architecture Domain BFF (`v0.4.0`) powering all **7 stateful Super-App verticals**:
    1. **Abeta Fresh** (`abeta-fresh` — 8-Min Quick-Commerce Grocery with sticky search, hero `MediaPagerAtom`, & Express `molecules`)
    2. **Abeta Bites** (`abeta-bites` — Food Delivery & Dining)
    3. **Abeta Pulse** (`abeta-pulse` — Live Events, Movies & Experiences)
    4. **Abeta Mart** (`abeta-mart` — Multi-Category Marketplace)
    5. **Abeta Spark** (`abeta-spark` — Swipe Discovery & Matchmaking)
    6. **Abeta Trade** (`abeta-trade` — B2B Wholesale & RFQ Procurement)
    7. **Abeta Pay** (`abeta-pay` — Zero-Fee P2P Wallet, Smart Metal Cards & Instant Settlement)
  - Open/Closed (`OCP`) `domain.VerticalModule` architecture, deterministic `snippets.EnsureComponentIDs`, batched analytics (`POST /api/v1/analytics/events`, `GET /api/v1/analytics/summary`), search/filter rendering, mutex-protected vertical & analytics stores, and `follow_up_actions` orchestration.
  - Dual-mode integration with `Abeta-dev/layout-engine` via `internal/infra/layoutclient` with seamless zero-dependency standalone fallback.
  - 1:1 wire parity with `Abeta-dev/kmp` (`github.com/abeta-dev/kmp/bff@v0.6.0` — 29 Wire Atoms / 31 Compose UI Atoms / 36 `ComposedNodeKind`s + `github.com/abeta-dev/go-libs@v0.4.0`), OpenAPI 3.1 + Scalar docs (`/openapi.json`, `/docs`), distroless Docker + 3-service Compose stack, and CI/CD security gates (`CI Quality Gate`).

## Non-Negotiable Architecture & PR Review Gates
1. **Strict Clean Architecture**: `internal/domain` must pass `internal/domain/arch_test.go` with zero Gin, HTTP, handler, usecase, vertical, or infra imports.
2. **OCP Vertical Composition**: New vertical apps must plug in via `domain.VerticalModule` without adding vertical switches to core usecases or handlers.
3. **1:1 KMP Wire Parity (`v0.6.0`)**: Every emitted `PageResponse`, `SnippetEnvelope`, `ActionTrigger`, `FloatingCartSpec`, `AdaptivePageSpec`, `analytics.*`, and `ginmw.Envelope` must use `github.com/abeta-dev/kmp/bff@v0.6.0` verbatim and be normalized via `snippets.EnsureComponentIDs`.
4. **KMP String-Only Tracking Properties**: Reject any PR that puts non-string values inside `TrackingContract.properties`.
5. **Race-Free Concurrency & Dual-Mode Resilience**: All stateful vertical stores must guard mutations and reads with mutexes (`go test -count=1 -race -cover ./...`), and `LAYOUT_ENGINE_URL` absence or failure must always degrade cleanly to standalone rendering.
6. **Security, DevOps & Brand Hygiene**: Workflows must pin minimal `permissions:` and `concurrency:`, scan commits using SHA-256-verified MIT `gitleaks` CLI `v8.30.1`, anchor `/sdui-bff` in `.gitignore`, and pass `./scripts/check_brand_hygiene.sh` and `./scripts/verify_all.sh`.
