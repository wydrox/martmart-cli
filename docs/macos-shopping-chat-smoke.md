# Manual live smoke: macOS Shopping Chat / MartMart

This smoke test touches live MartMart provider APIs but never finalizes checkout.

```bash
PROVIDER=frisco PRODUCT_QUERY='cola zero 330ml' PRODUCT_ID=154030 ./scripts/macos-shopping-chat-smoke.sh
```

Flow:

1. verify session
2. search product
3. optionally add a known product id to cart
4. show cart
5. checkout preview
6. stop — no finalize

For Delio, set `PROVIDER=delio` and use a Delio SKU as `PRODUCT_ID` if adding to cart.
