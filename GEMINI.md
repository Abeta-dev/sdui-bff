# GEMINI.md — Gemini, Antigravity & JetBrains AI Instructions for `abeta-dev/sdui-bff`

This repository is **Abeta SDUI Domain BFF (`github.com/abeta-dev/sdui-bff`, `v0.4.0`)**, a standalone Go `1.26.0` Clean Architecture Backend-for-Frontend powering **7 stateful vertical Super-Apps** for `Abeta-dev/kmp` (`github.com/abeta-dev/kmp/bff@v0.6.0` — **29 Wire Atoms / 31 Compose UI Atoms / 36 `ComposedNodeKind`s**, Express Quick-Commerce `molecules`, deterministic `EnsureComponentIDs`, and batched `analytics` endpoints `POST /api/v1/analytics/events` & `GET /api/v1/analytics/summary` + `github.com/abeta-dev/go-libs@v0.4.0`), with optional dynamic layout hydration from `Abeta-dev/layout-engine`.

Read [AGENTS.md](AGENTS.md) for the authoritative architectural contract. This file adapts routing for **Gemini CLI**, **Gemini Code Assist**, **Antigravity**, and **JetBrains AI**.

---

## Skill & Code Owner Routing

| Area | Guide / Skill | Code Owner Persona |
| :--- | :--- | :--- |
| Full SDUI BFF Development | [skills/abeta-sdui-bff/SKILL.md](skills/abeta-sdui-bff/SKILL.md) | [agents/bff-engineer.md](agents/bff-engineer.md) |
| Clean Architecture & Layout Engine Integration | [docs/architecture.md](docs/architecture.md) | [agents/bff-engineer.md](agents/bff-engineer.md) |
| 7 Stateful Vertical Apps & OCP Module Guide | [docs/verticals-guide.md](docs/verticals-guide.md) | [agents/bff-engineer.md](agents/bff-engineer.md) |

---

## Non-Negotiable Invariants

1. **Strict Clean Architecture**: `cmd/sdui-bff` -> `internal/handler` -> `internal/usecase` -> `internal/domain` <- `internal/infra` & `internal/verticals/*`. Zero HTTP, Gin, usecase, vertical, or infra imports inside `internal/domain` (`internal/domain/arch_test.go`).
2. **OCP `VerticalModule` Registry**: Every vertical app plugs in via `domain.VerticalModule` (`Descriptor`, `Screens`, `CanHandleScreen`, `RenderScreen`) — never hardcode vertical switches in core usecases or handlers.
3. **1:1 `abeta-dev/kmp` Wire Parity (`v0.6.0`)**: Emit typed `v1` contracts from `github.com/abeta-dev/kmp/bff/pkg/sdui/{actions,analytics,atoms,molecules,snippets,theming}` (**29 Wire Atoms / 31 Compose UI Atoms / 36 `ComposedNodeKind`s**) normalized via `snippets.EnsureComponentIDs` and wrapped in `github.com/abeta-dev/go-libs/ginmw` envelopes (`ginmw.RespondSuccess` / `ginmw.RespondError`).
4. **KMP String-Only Tracking Properties**: Every `TrackingContract.properties` map MUST contain string values only so KMP `kotlinx.serialization` decodes without type mismatch errors.
5. **Race-Free State & Dual-Mode Fallback**: Guard vertical stores and `analyticsstore.InMemoryStore` with mutexes (`go test -race ./...` green) and guarantee zero-dependency standalone operation when `LAYOUT_ENGINE_URL` is unset or unreachable.
6. **Brand, Secret, CI Gate & Path Hygiene**: Strictly use Abeta showcase brands (`Abeta Fresh`, `Abeta Bites`, `Abeta Pulse`, `Abeta Mart`, `Abeta Spark`, `Abeta Trade`, `Abeta Pay`) and zero machine-specific paths. CI scans commits with SHA-256 verified MIT `gitleaks` CLI `v8.30.1` and aggregates all jobs into **`CI Quality Gate`**.

---

## Verification Gates

```bash
./scripts/check_brand_hygiene.sh
./scripts/verify_all.sh
```
