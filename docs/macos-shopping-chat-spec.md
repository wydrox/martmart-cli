# MartMart Shopping Chat macOS MVP

## Objective

Build a very lightweight native macOS app that gives MartMart a fast graphical shopping surface: chat-driven product search, cart management, delivery slot planning, guarded checkout, order history, and local product/price history.

## Product scope

- Providers: Frisco and Delio first.
- Shopping assistant: `pi` turns user requests into explicit proposed shopping actions.
- Execution layer: local `martmart` performs provider operations: session status, search, product details, cart, reservation, checkout, and orders.
- Local history: read `~/.martmart-cli/catalog.db` for products, snapshots, and query history.
- UI style: native SwiftUI, minimal chrome, fast startup, no Electron, no remote app backend.

## MVP screens

1. **Chat**
   - User enters shopping requests in natural language.
   - `pi` replies with text plus proposed actions.
   - Product results render as small cards with image, provider, price, promo/deposit, availability, and add-to-cart controls.

2. **Cart**
   - Show current cart per provider.
   - Edit quantities, remove items, refresh totals.
   - Surface price changes, unavailable items, deposits, and promotions.

3. **Checkout**
   - Pick or confirm address and delivery slot through MartMart reservation tools.
   - Show checkout preview: items, item count, totals, delivery, payment state, warnings, and readiness.
   - Finalize headlessly through MartMart only after explicit user confirmation.

4. **Orders**
   - List previous orders with provider, date, status, total.
   - Drill into products, delivery, and payment details when available.

5. **Product History**
   - Search local catalog.
   - Show last seen products, current price, availability, and price snapshot history.

## Checkout safety rules

- No automatic finalization from chat.
- Finalize requires a fresh preview and an explicit click on **Zamów i zapłać**.
- Finalize sends guards derived from the preview: cart id, item count, total, reservation/payment identifiers where available.
- If the fresh preview differs from what the user approved, stop and show the diff.
- Hide/redact tokens, cookies, addresses, and payment details from logs and debug UI by default.

## 3DS / redirect handling

- If MartMart returns `action_required`, the app shows a focused handoff panel.
- Prefer an in-app WebView only for provider/payment URLs that MartMart marks safe to open.
- Otherwise open the system browser and show a **Sprawdź status zamówienia** button.
- After handoff, refresh order/cart state through MartMart rather than assuming success.

## Architecture

- SwiftUI + async/await.
- `MartMartClient`: local MCP/CLI bridge, cancellable requests, typed DTOs.
- `PiClient`: user prompt -> proposed actions -> natural-language response.
- `CatalogStore`: read-only SQLite access to MartMart catalog.
- `AppState`: selected provider, chat transcript, cart, checkout preview, loading/error state.

## Non-goals for MVP

- No Electron/web shell.
- No custom backend or cloud sync.
- No autonomous checkout.
- No broad provider abstraction beyond Frisco/Delio.
- No complex animations or heavyweight design system.
