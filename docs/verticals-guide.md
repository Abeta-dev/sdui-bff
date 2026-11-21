# Verticals Guide — 6 Stateful Super-App Modules

`Abeta-dev/sdui-bff` hosts **6 stateful vertical applications** implemented as Open/Closed (`OCP`) `domain.VerticalModule` packages under `internal/verticals/`. Every vertical uses `github.com/abeta-dev/kmp/bff@v0.2.0` contracts (including `search_bar`, `chip_group`, `swipe_deck`, `product_card`, `carousel`, `grid`, `tabs`, `accordion`, and `form_v1`) and emits rich `follow_up_actions` for `Abeta-dev/kmp`.

---

## 1. Vertical Catalog Summary

| Vertical | Package | Tenant ID | Theme Preset | Screen IDs | Mutation Endpoint(s) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Abeta Fresh** | `internal/verticals/fresh` | `abeta-fresh` | `EMERALD` | `fresh-home`, `store-home` | `POST /api/v1/fresh/orders/express`<br>`POST /api/v1/orders/express` |
| **Abeta Bites** | `internal/verticals/bites` | `abeta-bites` | `AMBER` | `bites-home`, `bites-restaurant`, `bites-order-track` | `POST /api/v1/bites/orders` |
| **Abeta Pulse** | `internal/verticals/pulse` | `abeta-pulse` | `INDIGO` | `pulse-home`, `pulse-event-detail` | `POST /api/v1/pulse/bookings` |
| **Abeta Mart** | `internal/verticals/mart` | `abeta-mart` | `AMBER` | `mart-home`, `mart-product-detail` | `POST /api/v1/mart/orders` |
| **Abeta Spark** | `internal/verticals/spark` | `abeta-spark` | `INDIGO` | `spark-deck`, `spark-matches` | `POST /api/v1/spark/swipe` |
| **Abeta Trade** | `internal/verticals/trade` | `abeta-trade` | `EMERALD` | `trade-home`, `trade-supplier` | `POST /api/v1/trade/rfq` |

---

## 2. Vertical-by-Vertical Reference

### 2.1 Abeta Fresh — 8-Min Quick-Commerce Grocery (`abeta-fresh`)
- **Screens**:
  - `fresh-home` (plus `store-home` compatibility alias): Live grocery search (`search_bar`), category filter pills (`chip_group`), promo banners, responsive `product_card` grids with `StepperAtom` quantity selectors, and a sticky `FloatingCartSpec` checkout dock.
- **Interactive Mutations**:
  - `POST /api/v1/fresh/orders/express` (and `POST /api/v1/orders/express`): Records an express grocery order in mutex-protected vertical state and returns `trigger_feedback` (`effect: "celebration"`) + `show_toast` follow-up actions.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/fresh-home?q=avocado&city=Bengaluru"

  curl -X POST http://localhost:8085/api/v1/fresh/orders/express \
    -H "Content-Type: application/json" \
    -d '{"vertical":"abeta-fresh","item_count":3,"total_usd":14.50,"address":"Indiranagar, Bengaluru"}'
  ```

### 2.2 Abeta Bites — Food Delivery & Dining (`abeta-bites`)
- **Screens**:
  - `bites-home`: Cuisine filter chips (`chip_group`), restaurant search (`search_bar`), curated meal carousels, and top-rated restaurant cards.
  - `bites-restaurant`: Menu sections, customizable dish `product_card` items, ratings, and floating meal cart.
  - `bites-order-track`: Live meal preparation and courier ETA status timeline backed by placed order state.
- **Interactive Mutations**:
  - `POST /api/v1/bites/orders`: Persists the meal order and returns celebration feedback, toast notification, and a `navigate` follow-up action directly to `bites-order-track`.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/bites-home?filter=artisan"

  curl -X POST http://localhost:8085/api/v1/bites/orders \
    -H "Content-Type: application/json" \
    -d '{"vertical":"abeta-bites","item_count":2,"total_usd":22.00,"address":"Koramangala, Bengaluru"}'
  ```

### 2.3 Abeta Pulse — Live Events, Movies & Experiences (`abeta-pulse`)
- **Screens**:
  - `pulse-home`: City-scoped event discovery with `search_bar`, category `chip_group` (Concerts, Cinema, Comedy, Dining), and featured weekend event carousels.
  - `pulse-event-detail`: Venue metadata, seat-tier inventory (`accordion` / `tabs`), and one-tap ticket reservation actions.
- **Interactive Mutations**:
  - `POST /api/v1/pulse/bookings`: Validates seat tier & quantity (`domain.BookingRequest`), updates remaining booking counts, and emits a celebration + QR pass toast.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/pulse-home?city=Mumbai"

  curl -X POST http://localhost:8085/api/v1/pulse/bookings \
    -H "Content-Type: application/json" \
    -d '{"event_id":"neon_horizon_symphony","seat_tier":"arena_gold","quantity":2,"city":"Bengaluru"}'
  ```

### 2.4 Abeta Mart — Multi-Category Marketplace (`abeta-mart`)
- **Screens**:
  - `mart-home`: Multi-department storefront with `search_bar`, department `chip_group`, flash deal banners, and responsive multi-column `product_card` catalog (`ResponsiveColumnsSpec`).
  - `mart-product-detail`: Spec comparison tabs, warranty accordion, fulfillment badges, and instant checkout triggers.
- **Interactive Mutations**:
  - `POST /api/v1/mart/orders`: Places a marketplace order and returns dispatch confirmation feedback + toast.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/mart-home?q=headphones"

  curl -X POST http://localhost:8085/api/v1/mart/orders \
    -H "Content-Type: application/json" \
    -d '{"vertical":"abeta-mart","item_count":1,"total_usd":129.00,"address":"HSR Layout, Bengaluru"}'
  ```

### 2.5 Abeta Spark — Swipe Discovery & Matchmaking (`abeta-spark`)
- **Screens**:
  - `spark-deck`: Full interactive `swipe_deck` (`SwipeDeckSnippetData`) paired with interest filter `chip_group` and quick-action row (`nope`, `super_like`, `like`, `undo`).
  - `spark-matches`: Stateful list of mutual `SparkMatch` entries updated live as users swipe.
- **Interactive Mutations**:
  - `POST /api/v1/spark/swipe`: Applies a swipe decision (`like`, `nope`, `super_like`, `undo`) on the mutex-protected deck state. Mutual likes persist a `SparkMatch` and emit an *"It's a Match!"* celebration overlay; passes emit a subtle info toast.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/spark-deck"

  curl -X POST http://localhost:8085/api/v1/spark/swipe \
    -H "Content-Type: application/json" \
    -d '{"profile_id":"spark_maya","direction":"like"}'
  ```

### 2.6 Abeta Trade — B2B Wholesale & RFQ Procurement (`abeta-trade`)
- **Screens**:
  - `trade-home`: Verified supplier directory with `search_bar`, industrial category `chip_group`, MOQ pricing tiers, and active RFQ summary metrics.
  - `trade-supplier`: Supplier credentials, tiered wholesale pricing tables, and an interactive `form_v1` Request For Quotation submission form.
- **Interactive Mutations**:
  - `POST /api/v1/trade/rfq`: Validates `domain.RFQRequest`, stores a persisted `domain.RFQRecord` with a generated `quote_id`, and emits celebration + confirmation toast follow-up actions.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/trade-home?filter=packaging"

  curl -X POST http://localhost:8085/api/v1/trade/rfq \
    -H "Content-Type: application/json" \
    -d '{"supplier_id":"apex_corrugated","product_name":"Corrugated Shipping Boxes","quantity":500,"company_name":"Northstar Retail","contact_email":"procurement@northstar.example"}'
  ```

---

## 3. Adding a New Vertical (`OCP` Recipe)

Because screen discovery and dispatch rely exclusively on `domain.VerticalModule`, adding a 7th vertical requires zero edits to `internal/domain` or `internal/usecase`:

1. **Create package `internal/verticals/<name>/`** with a struct implementing `domain.VerticalModule`:
   ```go
   // Compile-time port check:
   var _ domain.VerticalModule = (*Module)(nil)
   ```
2. **Guard mutable state with `sync.RWMutex`** so concurrent HTTP requests and `go test -race ./...` are strictly race-free.
3. **Use `github.com/abeta-dev/kmp/bff@v0.2.0` builders** (`snippets`, `atoms`, `actions`, `theming`) and honor the **string-only tracking property rule** (`map[string]string` in every `TrackingContract`).
4. **Register the new module** in `cmd/sdui-bff` when constructing the `ScreenComposer` (and mount any vertical-specific mutation routes in `internal/handler`).
5. **Write table-driven unit & concurrency tests** (`<name>_test.go`) verifying all screens, search/filter queries, mutation endpoints, `follow_up_actions`, and tracking property types.
6. **Run the full verification gate**:
   ```bash
   ./scripts/verify_all.sh
   ```
