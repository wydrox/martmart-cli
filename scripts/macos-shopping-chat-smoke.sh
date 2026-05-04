#!/usr/bin/env bash
set -euo pipefail

# Manual live smoke for the macOS Shopping Chat / MartMart integration.
# It intentionally never finalizes checkout.
# Usage: PROVIDER=frisco PRODUCT_QUERY='cola zero 330ml' PRODUCT_ID=154030 ./scripts/macos-shopping-chat-smoke.sh

PROVIDER="${PROVIDER:-frisco}"
PRODUCT_QUERY="${PRODUCT_QUERY:-cola zero 330ml}"
PRODUCT_ID="${PRODUCT_ID:-}"
QTY="${QTY:-1}"
MARTMART="${MARTMART:-./martmart}"

echo "== session verify ($PROVIDER) =="
"$MARTMART" --provider "$PROVIDER" session verify

echo "== product search: $PRODUCT_QUERY =="
"$MARTMART" --provider "$PROVIDER" --format json products search --search "$PRODUCT_QUERY" --page-size 5

if [[ -n "$PRODUCT_ID" ]]; then
  echo "== add cart: $PRODUCT_ID x $QTY =="
  "$MARTMART" --provider "$PROVIDER" --format json cart add --product-id "$PRODUCT_ID" --quantity "$QTY"
else
  echo "Skipping cart add: set PRODUCT_ID to add a real item."
fi

echo "== cart show =="
"$MARTMART" --provider "$PROVIDER" --format json cart show

echo "== checkout preview (NO FINALIZE) =="
"$MARTMART" --provider "$PROVIDER" --format json checkout preview

echo "Smoke complete. No checkout finalize was called."
