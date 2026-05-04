#!/usr/bin/env bash
set -euo pipefail

MARTMART="${MARTMART:-./martmart}"
QUERY="${QUERY:-cola zero 330ml}"

echo "== release build =="
swift build --package-path macos-app -c release >/dev/null
APP="macos-app/.build/release/MartMartShoppingChat"

echo "== binary size =="
ls -lh "$APP"

echo "== cold self-check =="
/usr/bin/time -l "$APP" --self-check >/tmp/martmart-shopping-chat-self-check.out
cat /tmp/martmart-shopping-chat-self-check.out

echo "== first MartMart search =="
/usr/bin/time -l "$MARTMART" --provider frisco --format json products search --search "$QUERY" --page-size 5 >/tmp/martmart-shopping-chat-search.json
python3 - <<'PY'
import json
p='/tmp/martmart-shopping-chat-search.json'
try:
    data=json.load(open(p))
    print('products:', len(data.get('products', [])), 'total:', data.get('totalCount'))
except Exception as e:
    print('could not summarize search:', e)
PY
