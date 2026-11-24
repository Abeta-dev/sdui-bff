# Verticals Guide — 7 Stateful Super-App Modules

`Abeta-dev/sdui-bff` (`v0.4.0`) hosts **7 stateful vertical applications** implemented as Open/Closed (`OCP`) `domain.VerticalModule` packages under `internal/verticals/`. Every vertical uses `github.com/abeta-dev/kmp/bff@v0.6.0` contracts — all **21 canonical SDUI snippet types** (including sticky `search_bar`, `chip_group`, `swipe_deck`, `product_card`, `carousel`, `grid`, `tabs`, `accordion`, `form_v1`, plus `nudge_bar`, `marquee_strip`, animated 4-step `order_tracker`, and server-molded `composed_block` with **36 `ComposedNodeKind`s** + reusable `pkg/sdui/molecules` Express Quick-Commerce molecules), **29 Wire Atoms / 31 Compose UI Atoms** (incl. `CountdownAtom`, `MediaPagerAtom`, & the 12 extended atoms `QrCode`, `OtpInput`, `Select`, `SegmentedControl`, `RadioGroup`, `Rating`, `Price`, `Badge`, `AlertBanner`, `StatTile`, `KeyValueRow`, `Chip`), deterministic **`snippets.EnsureComponentIDs`** applied by `ScreenComposer.GetScreen` across 100% of screens, **7 ambient `MotionLoopEffect`s**, Free-Delivery threshold `FloatingCartSpec` docks, batched analytics (`POST /api/v1/analytics/events` & `GET /api/v1/analytics/summary`), and rich `follow_up_actions` for `Abeta-dev/kmp`.

---

## 1. Vertical Catalog Summary

| Vertical | Package | Tenant ID | Theme Preset | Screen IDs | v0.6.0 Molded Atoms, Molecules & Snippets | Mutation Endpoint(s) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Abeta Fresh** | `internal/verticals/fresh` | `abeta-fresh` | `EMERALD` | `fresh-home`, `store-home` | Sticky `search_bar`, hero `MediaPagerAtom`, `CategoryGridSection` (3/4/6), `ExpressProductTile` w/ `box_alignment` ribbons + `VariantPickerSheetAction` bottom sheet, `ImpulseUpsellRail`, `DeliveryInstructionsCard`, `marquee_strip`, `nudge_bar` (`420s` countdown), `MoldLoyaltyStampRecipe`, molded `Select` delivery slot + `KeyValueRow` savings + `Badge`, `$15` cart dock | `POST /api/v1/fresh/orders/express`<br>`POST /api/v1/orders/express` |
| **Abeta Bites** | `internal/verticals/bites` | `abeta-bites` | `AMBER` | `bites-home`, `bites-restaurant`, `bites-order-track` | `marquee_strip`, molded `Rating` + `Price` + `RadioGroup` speed + `KeyValueRow`, `$18` cart dock, animated 4-step `order_tracker` (`#BTS-xxxx`) | `POST /api/v1/bites/orders` |
| **Abeta Pulse** | `internal/verticals/pulse` | `abeta-pulse` | `INDIGO` | `pulse-home`, `pulse-event-detail` | Early-bird `nudge_bar` (`540s` countdown), molded `AlertBanner` + `Rating` + `QrCode` mobile e-pass preview | `POST /api/v1/pulse/bookings` |
| **Abeta Mart** | `internal/verticals/mart` | `abeta-mart` | `INDIGO` | `mart-home`, `mart-product-detail` | `marquee_strip`, `MoldFlashDealRecipe`, molded `Price` (MRP + discount pill) + `Rating` + `Chip` + `KeyValueRow`, `$25` cart dock | `POST /api/v1/mart/orders` |
| **Abeta Spark** | `internal/verticals/spark` | `abeta-spark` | `AMBER` | `spark-deck`, `spark-matches` | Prime Boost `nudge_bar` (`heartbeat` badge), interactive `swipe_deck` | `POST /api/v1/spark/swipe` |
| **Abeta Trade** | `internal/verticals/trade` | `abeta-trade` | `EMERALD` | `trade-home`, `trade-supplier` | Molded MOQ Bulk Savings (`RollingCounter` + `StatTile` telemetry), landed-cost `StatTile` + `KeyValueRow` breakdown | `POST /api/v1/trade/rfq` |
| **Abeta Pay** | `internal/verticals/pay` | `abeta-pay` | `INDIGO` | `pay-home`, `pay-transfer`, `pay-cards`, `pay-receipt` | Wallet `StatTile`s + `AlertBanner` + `Badge`, `MarqueeStrip`, live `#PAY-3081` `OrderTracker`, scan-to-pay `QrCode` + `Chip`, `SegmentedControl` + `RadioGroup` + masked `OtpInput` + `KeyValueRow` transfer flow, card controls, itemized receipt + OS `share` | `POST /api/v1/pay/transfers`<br>`POST /api/v1/pay/cards/controls` |

---

## 2. Vertical-by-Vertical Reference

### 2.1 Abeta Fresh — 8-Min Quick-Commerce Grocery (`abeta-fresh`)
- **Screens**:
  - `fresh-home` (plus `store-home` compatibility alias): Showcases the complete Atomic -> Molecular -> Compound SDUI architecture with express perks `marquee_strip` (`"8-min dispatch"`, `"Free delivery over $15"`, `"Zero surge pricing"`), **sticky** live grocery search (`search_bar` with `WithSticky(true)`), auto-advancing hero `MediaPagerAtom` (`express_hero_media_pager`), category filter pills (`chip_group`), reusable `pkg/sdui/molecules` sections (`CategoryGridSection` responsive 3/4/6 grid, `ExpressProductTile` bestsellers with `box_alignment` discount/ETA/favorite ribbons + `VariantPickerSheetAction` bottom-sheet pack selector, `ImpulseUpsellRail` horizontal `ScrollRowNode`, and `DeliveryInstructionsCard` with switch/checkbox/text field/tip chips/slider), urgency `nudge_bar` (`fresh_flash_nudge` with `420s` `CountdownAtom` + `MotionLoopWiggle` badge), promo banners, responsive `product_card` grids with `StepperAtom` quantity selectors, server-molded `composed_block` (`snippets.MoldLoyaltyStampRecipe` stateful punch-card + `express_delivery_summary`), deterministic `snippets.EnsureComponentIDs`, and a sticky `FloatingCartSpec` checkout dock with `$15` (`1500` cents) Free-Delivery threshold nudge (`UnlockedLabel: "FREE 8-min delivery unlocked!"`).
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

### 2.7 Abeta Pay — Fintech Wallet & Smart Cards (`abeta-pay`)
- **Screens**:
  - `pay-home`: Wallet & Cashback `StatTile` telemetry + `Badge` + `AlertBanner`, zero-fee perks `marquee_strip`, live `#PAY-3081` 4-stage `order_tracker`, molded scan-to-pay `QrCode` + quick-pay `Chip` card, and searchable instant P2P contacts (`@maya.chen`, `@arjun.rao`, `@elena.m`, `@noah.park`).
  - `pay-transfer`: Molded `composed_block` featuring `SegmentedControl` (Instant / Scheduled / AutoPay), `RadioGroup` (Checking / Cashback Wallet / Reserve), masked 4-digit `OtpInput` Transaction PIN, `KeyValueRow` `$0.00` fee & `5%` cashback breakdown, validated `form_v1`, and OS Biometric Auth row.
  - `pay-cards`: Smart metal virtual card (`•••• 4092`) controls with switches (Freeze, International FX, Contactless), spend limit slider (`$500`–`$15,000`), molded `Badge` status + `AlertBanner` security shield + `KeyValueRow` cap summary.
  - `pay-receipt`: Live 4-stage `order_tracker`, molded `composed_block` with settled `AlertBanner`, itemized `KeyValueRow` receipt (`#PAY-3081`, recipient, settlement rail, `$0.00` fee, `+$6.25` 5% cashback), verification `QrCode`, and native OS `share` action.
- **Interactive Mutations**:
  - `POST /api/v1/pay/transfers`: Validates `domain.TransferRequest`, updates wallet balance and 5% cashback vault, stores a `domain.TransferRecord`, and emits `trigger_feedback` (`celebration`) + `show_toast` + `navigate` (`pay-receipt?id=PAY-xxxx`).
  - `POST /api/v1/pay/cards/controls`: Validates `domain.CardControlRequest`, updates card freeze/FX/contactless/spend-limit state atomically, and emits `trigger_feedback` + `show_toast` + string-only `track_event`.
- **Example**:
  ```bash
  curl "http://localhost:8085/api/v1/screens/pay-home"

  curl -X POST http://localhost:8085/api/v1/pay/transfers \
    -H "Content-Type: application/json" \
    -d '{"recipient":"@maya.chen","amount_usd":125.00,"speed":"instant","funding_source":"checking","pin":"4829"}'

  curl -X POST http://localhost:8085/api/v1/pay/cards/controls \
    -H "Content-Type: application/json" \
    -d '{"card_id":"metal_4092","frozen":false,"international":true,"contactless":true,"spend_limit_usd":6000}'
  ```

---

## 3. Adding a New Vertical (`OCP` Recipe)

Because screen discovery and dispatch rely exclusively on `domain.VerticalModule`, adding an 8th vertical requires zero edits to `internal/usecase`:

1. **Create package `internal/verticals/<name>/`** with a struct implementing `domain.VerticalModule`:
   ```go
   // Compile-time port check:
   var _ domain.VerticalModule = (*Module)(nil)
   ```
2. **Guard mutable state with `sync.RWMutex`** so concurrent HTTP requests and `go test -race ./...` are strictly race-free.
3. **Use `github.com/abeta-dev/kmp/bff@v0.6.0` builders** (`snippets`, `molecules`, `atoms`, `actions`, `analytics`, `theming` — including `MediaPagerAtom`, the 12 extended production atoms, `nudge_bar`, `marquee_strip`, `order_tracker`, and `composed_block` molding helpers across 36 `ComposedNodeKind`s; `ScreenComposer.GetScreen` applies `snippets.EnsureComponentIDs` automatically) and honor the **string-only tracking property rule** (`map[string]string` in every `TrackingContract`).
4. **Register the new module** in `cmd/sdui-bff` when constructing the `ScreenComposer` (and mount any vertical-specific mutation routes in `internal/handler`).
5. **Write table-driven unit & concurrency tests** (`<name>_test.go`) verifying all screens, search/filter queries, mutation endpoints, `follow_up_actions`, and tracking property types.
6. **Run the full verification gate**:
   ```bash
   ./scripts/verify_all.sh
   ```
