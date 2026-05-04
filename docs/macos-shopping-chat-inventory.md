# MartMart capability inventory for macOS Shopping Chat

## Existing CLI commands

- `session`: `list`, `verify`, `login`, `from-curl`, `refresh-token`.
- `products`: `search`, `by-ids`, `get`, `nutrition`, `pick`.
- `cart`: `show`, `add`, `add-batch`, `remove`, `remove-batch`.
- `reservation`: `delivery-options`, `calendar`, `slots`, `reserve`, `plan`, `cancel`.
- `checkout`: `preview`, `finalize --confirm` for Frisco/Delio; guarded and experimental.
- `orders`: `list`, `get`, `delivery`, `payments`, `products`.
- `account`: profile, addresses, consents, vouchers, payments, membership.

## Existing MCP tools

- Session/account: `session_status`, `session_login`, `session_from_curl`, `session_refresh_token`, `providers_list`, account/address/payment/voucher/membership tools.
- Products/cart: `products_search`, `products_by_ids`, `products_nutrition`, `cart_show`, `cart_add`, `cart_remove`.
- Orders/reservation: `orders_list`, `orders_details`, `orders_delivery`, `orders_payments`, `reservation_delivery_options`, `reservation_calendar`, `reservation_slots`, `reservation_reserve`, `reservation_plan`, `reservation_cancel`.
- UpMenu public tools are separate and out of MVP scope.

## Gap for macOS app MVP

- MCP currently lacks checkout tools equivalent to CLI `checkout preview` and guarded `checkout finalize`.
- Product/cart/checkout outputs are provider-shaped enough for agents, but the app needs stable lightweight DTOs for UI cards and totals.
- Need test fixtures for UI/client work without live provider calls.

## Recommended next implementation order

1. Add MCP `checkout_preview`.
2. Add MCP `checkout_finalize` with `confirm` and guard inputs.
3. Add/define normalized DTO wrappers for ProductCard, CartSummary, CheckoutPreview/Finalize.
4. Add fixtures for Frisco/Delio search, cart, reservation slots, checkout preview/finalize, orders.
