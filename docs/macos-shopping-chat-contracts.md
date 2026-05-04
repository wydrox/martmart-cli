# macOS Shopping Chat DTO contracts

These are the app-facing lightweight shapes. Provider raw payloads may still be attached under `raw` for diagnostics, but UI should render from these fields only.

## ProductCard

```json
{
  "id": "154030",
  "provider": "frisco",
  "sku": "154030",
  "name": "Coca-Cola Zero 330 ml",
  "brand": "COCA-COLA",
  "image_url": "https://...",
  "price": { "amount": 4.59, "currency": "PLN" },
  "promo_price": { "amount": 3.49, "currency": "PLN", "condition": "when buying 6" },
  "deposit": { "amount": 0.50, "currency": "PLN" },
  "measure": { "value": 330, "unit": "ml", "display": "330 ml" },
  "available": true,
  "available_quantity": 3761,
  "category": "Napoje > Gazowane > Cola > Bez cukru",
  "raw": {}
}
```

Required for rendering: `id`, `provider`, `name`, `price`, `available`. Images and promo/deposit are optional.

## CartSummary

```json
{
  "provider": "frisco",
  "cart_id": "cart-1",
  "items": [
    {
      "product": { "id": "154030", "provider": "frisco", "name": "Coca-Cola Zero", "image_url": "https://..." },
      "quantity": 6,
      "unit_price": { "amount": 4.59, "currency": "PLN" },
      "promo_unit_price": { "amount": 3.49, "currency": "PLN" },
      "line_total": { "amount": 20.94, "currency": "PLN" },
      "deposit_total": { "amount": 3.00, "currency": "PLN" },
      "available": true,
      "warnings": []
    }
  ],
  "totals": {
    "items": { "amount": 20.94, "currency": "PLN" },
    "delivery": { "amount": 9.99, "currency": "PLN" },
    "deposits": { "amount": 3.00, "currency": "PLN" },
    "grand": { "amount": 33.93, "currency": "PLN" }
  },
  "warnings": [],
  "raw": {}
}
```

## CheckoutPreview

Use MartMart `checkout.CheckoutPreview` as the canonical normalized shape:

```json
{
  "provider": "frisco",
  "user_id": "646456",
  "cart_id": "cart-1",
  "item_count": 6,
  "total": { "amount": 33.93, "currency": "PLN" },
  "reservation": { "starts_at": "2026-05-05T10:00:00+02:00", "ends_at": "2026-05-05T12:00:00+02:00" },
  "payment": { "method": "CARD", "channel": "Adyen", "status": "ready" },
  "ready_to_finalize": true,
  "issues": [],
  "raw": {}
}
```

## CheckoutFinalizeResult

Use MartMart `checkout.FinalizeResult` as the canonical normalized shape:

```json
{
  "provider": "frisco",
  "user_id": "646456",
  "status": "placed",
  "order_id": "ord-123",
  "preview": {},
  "action": null,
  "readback": { "order_id": "ord-123", "order": {}, "payments": [] },
  "api_response": {}
}
```

Allowed statuses: `placed`, `pending`, `requires_action`, `unknown`.

## ActionRequired

```json
{
  "status": "requires_action",
  "action": {
    "kind": "3ds",
    "url": "https://...",
    "method": "GET",
    "message": "Complete bank authentication",
    "payload": {}
  }
}
```

The macOS app must treat this as an unfinished checkout and then refresh order/cart state after the handoff.
