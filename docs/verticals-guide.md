# Verticals Guide — 6 Stateful Super-App Modules

`Abeta-dev/sdui-bff` hosts **6 stateful vertical applications** implemented as Open/Closed (`OCP`) `domain.VerticalModule` packages under `internal/verticals/`. Every vertical uses `github.com/abeta-dev/kmp/bff@v0.3.0` contracts — all **21 canonical SDUI snippet types** (including `search_bar`, `chip_group`, `swipe_deck`, `product_card`, `carousel`, `grid`, `tabs`, `accordion`, `form_v1`, plus Wave 6 `nudge_bar`, `marquee_strip`, animated 4-step `order_tracker`, and server-molded `composed_block` recipes), **16 Wire Atoms** (incl. `CountdownAtom`), **7 ambient `MotionLoopEffect`s**, Free-Delivery threshold `FloatingCartSpec` docks, and rich `follow_up_actions` for `Abeta-dev/kmp`.

---

## 1. Vertical Catalog Summary

| Vertical | Package | Tenant ID | Theme Preset | Screen IDs | Wave 6 Snippets & Nudges | Mutation Endpoint(s) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Abeta Fresh** | `internal/verticals/fresh` | `abeta-fresh` | `EMERALD` | `fresh-home`, `store-home` | `marquee_strip`, `nudge_bar` (`420s` countdown + `wiggle`), `composed_block` (`MoldLoyaltyStampRecipe`), `$15` Free-Delivery cart dock | `POST /api/v1/fresh/orders/express`<br>`POST /api/v1/orders/express` |
| **Abeta Bites** | `internal/verticals/bites` | `abeta-bites` | `AMBER` | `bites-home`, `bites-restaurant`, `bites-order-track` | `marquee_strip`, `$18` Free Priority Delivery cart dock, animated 4-step `order_tracker` (`#BTS-xxxx`, `660s` ETA countdown) | `POST /api/v1/bites/orders` |
| **Abeta Pulse** | `internal/verticals/pulse` | `abeta-pulse` | `INDIGO` | `pulse-home`, `pulse-event-detail` | Early-bird urgency `nudge_bar` (`540s` countdown + `wiggle`) | `POST /api/v1/pulse/bookings` |
| **Abeta Mart** | `internal/verticals/mart` | `abeta-mart` | `INDIGO` | `mart-home`, `mart-product-detail` | `marquee_strip`, `composed_block` (`MoldFlashDealRecipe`), `$25` Free-Delivery cart dock | `POST /api/v1/mart/orders` |
| **Abeta Spark** | `internal/verticals/spark` | `abeta-spark` | `AMBER` | `spark-deck`, `spark-matches` | Prime Boost `nudge_bar` (`heartbeat` badge) | `POST /api/v1/spark/swipe` |
| **Abeta Trade** | `internal/verticals/trade` | `abeta-trade` | `EMERALD` | `trade-home`, `trade-supplier` | MOQ Bulk Savings `composed_block` (`CardNode` + `VStack` + `HStack` + `RollingCounterNode`) | `POST /api/v1/trade/rfq` |

---

## 2. Vertical-by-Vertical Reference

### 2.1 Abeta Fresh — 8-Min Quick-Commerce Grocery (`abeta-fresh`)
- **Screens**:
  - `fresh-home` (plus `store-home` compatibility alias): Express perks `marquee_strip` (`"8-min dispatch"`, `"Free delivery over $15"`, `"Zero surge pricing"`), live grocery search (`search_bar`), category filter pills (`chip_group`), urgency `nudge_bar` (`fresh_flash_nudge` with `420s` `CountdownAtom` + `MotionLoopWiggle` badge), promo banners, responsive `product_card` grids with `StepperAtom` quantity selectors, server-molded `composed_block` (`snippets.MoldLoyaltyStampRecipe` stateful punch-card), and a sticky `FloatingCartSpec` checkout dock with `$15` (`1500` cents) Free-Delivery threshold nudge (`UnlockedLabel: "FREE 8-min delivery unlocked!"`).
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
  - `bites-home`: Hot meal offers `marquee_strip`, cuisine filter chips (`chip_group`), restaurant search (`search_bar`), curated meal carousels, and top-rated restaurant cards.
  - `bites-restaurant`: Menu sections, customizable dish `product_card` items, ratings, and floating meal cart with `$18` (`1800` cents) Free Priority Delivery threshold nudge (`UnlockedLabel: "FREE Priority Delivery unlocked!"`).
  - `bites-order-track`: Animated 4-step `order_tracker` (`snippets.NewOrderTrackerSnippet`) displaying the live `#BTS-xxxx` order code, ticking `660s` `CountdownAtom` ETA, 4 milestones (`Order Confirmed [completed]`, `Kitchen Preparing [completed]`, `Rider En Route [active]`, `Delivered`), rider details, and Call Rider / Back to Restaurants CTAs backed by placed order state.
- **Interactive Mutations**:
  - `POST /api/v1/bites/orders`: Persists the meal order and returns celebration feedback, toast notification, and a `navigate` follow-up action directly to `bites-order-track`.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/bites-home?filter=veg"

  curl -X POST http://localhost:8085/api/v1/bites/orders \
    -H "Content-Type: application/json" \
    -d '{"vertical":"abeta-bites","item_count":2,"total_usd":22.00,"address":"Koramangala, Bengaluru"}'
  ```

### 2.3 Abeta Pulse — Live Events, Movies & Experiences (`abeta-pulse`)
- **Screens**:
  - `pulse-home`: City-scoped event discovery with `search_bar`, category `chip_group` (Movies, Concerts, Comedy, Dining), urgency `nudge_bar` (`pulse_early_bird_nudge` with `540s` `CountdownAtom` `"Early-bird seats closing in"` + `MotionLoopWiggle` badge), spotlight banner, and responsive curated event grid.
  - `pulse-event-detail`: Venue metadata, seat-tier inventory cards, bottom bar CTA, and native OS `share` action.
- **Interactive Mutations**:
  - `POST /api/v1/pulse/bookings`: Validates seat tier & quantity (`domain.BookingRequest`), updates remaining booking counts, and emits a celebration + QR pass toast.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/pulse-home?city=Mumbai"

  curl -X POST http://localhost:8085/api/v1/pulse/bookings \
    -H "Content-Type: application/json" \
    -d '{"event_id":"evt-synth-symphony","seat_tier":"Gold","quantity":2,"city":"Bengaluru"}'
  ```

### 2.4 Abeta Mart — Multi-Category Marketplace (`abeta-mart`)
- **Screens**:
  - `mart-home`: Multi-department storefront with deals `marquee_strip`, `search_bar`, department `chip_group`, server-molded `composed_block` Flash Deal (`snippets.MoldFlashDealRecipe` with live countdown & rolling odometer price), lightning deals carousel, responsive multi-column `product_card` catalog (`ResponsiveColumnsSpec`), and sticky `FloatingCartSpec` with `$25` (`2500` cents) Free-Delivery threshold nudge.
  - `mart-product-detail`: Spec comparison & review accordions, fulfillment badges, native `share`, and instant Buy Now checkout triggers.
- **Interactive Mutations**:
  - `POST /api/v1/mart/orders`: Places a marketplace order and returns dispatch confirmation feedback + toast.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/mart-home?q=audio"

  curl -X POST http://localhost:8085/api/v1/mart/orders \
    -H "Content-Type: application/json" \
    -d '{"vertical":"abeta-mart","item_count":1,"total_usd":129.00,"address":"HSR Layout, Bengaluru"}'
  ```

### 2.5 Abeta Spark — Swipe Discovery & Matchmaking (`abeta-spark`)
- **Screens**:
  - `spark-deck`: Prime Boost `nudge_bar` (`spark_prime_boost` with `MotionLoopHeartbeat` badge), full interactive `swipe_deck` (`SwipeDeckSnippetData`) paired with interest filter `chip_group` and quick actions (`nope`, `super_like`, `like`, `undo`, `reset`).
  - `spark-matches`: Stateful list of mutual `SparkMatch` entries updated live as users swipe.
- **Interactive Mutations**:
  - `POST /api/v1/spark/swipe`: Applies a swipe decision (`like`, `nope`, `super_like`, `undo`, `reset`) on the mutex-protected deck state. Mutual likes persist a `SparkMatch` and emit an *"It's a Match!"* celebration overlay; passes emit a subtle info toast.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/spark-deck"

  curl -X POST http://localhost:8085/api/v1/spark/swipe \
    -H "Content-Type: application/json" \
    -d '{"profile_id":"spk_aanya","direction":"like"}'
  ```

### 2.6 Abeta Trade — B2B Wholesale & RFQ Procurement (`abeta-trade`)
- **Screens**:
  - `trade-home`: Verified supplier directory with `search_bar`, industrial category `chip_group`, server-molded `composed_block` MOQ Bulk Savings card (built declaratively via `snippets.CardNode`, `snippets.VStack`, `snippets.HStack`, and `snippets.RollingCounterNode`), Trade Assurance banner, and supplier cards with MOQ pricing tiers.
  - `trade-supplier`: Supplier credentials, TrustScore metric card, tiered wholesale pricing cards, an interactive `form_v1` Request For Quotation submission form, and recently submitted RFQs.
- **Interactive Mutations**:
  - `POST /api/v1/trade/rfq`: Validates `domain.RFQRequest`, stores a persisted `domain.RFQRecord` with a generated `quote_id`, and emits celebration + confirmation toast follow-up actions.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/trade-home?filter=packaging"

  curl -X POST http://localhost:8085/api/v1/trade/rfq \
    -H "Content-Type: application/json" \
    -d '{"supplier_id":"trd_ecopack","product_name":"Heavy-Duty 5-Ply Export Cartons","quantity":500,"company_name":"Northstar Retail","contact_email":"procurement@northstar.example"}'
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
3. **Use `github.com/abeta-dev/kmp/bff@v0.3.0` builders** (`snippets`, `atoms`, `actions`, `theming` — including Wave 6 `nudge_bar`, `marquee_strip`, `order_tracker`, and `composed_block` molding helpers) and honor the **string-only tracking property rule** (`map[string]string` in every `TrackingContract`).
4. **Register the new module** in `cmd/sdui-bff` when constructing the `ScreenComposer` (and mount any vertical-specific mutation routes in `internal/handler`).
5. **Write table-driven unit & concurrency tests** (`<name>_test.go`) verifying all screens, search/filter queries, mutation endpoints, `follow_up_actions`, and tracking property types.
6. **Run the full verification gate**:
   ```bash
   ./scripts/verify_all.sh
   ```
