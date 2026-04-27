---
name: meal-planner
description: Plan meals, search Frisco for matching products, build a cost-effective cart with nutrition tracking, and iterate with the user to optimize.
---

# Frisco Meal Planner

Help the user go from meal ideas or dietary guidelines to a filled grocery cart with full nutritional breakdown.

## Tooling / Auth Policy

- If the user asks to finish the task via MCP, use MCP end-to-end.
- If MCP is broken, fix MCP and continue via MCP instead of silently finishing the shopping flow via CLI.
- Run `session_status` for the chosen provider before any login action.
- Do **not** run `session_login` just because the session looks suspicious while `session_status` still reports an authenticated session.
- First try the actual MCP/API operation. Only suggest or run `session_login` after a concrete auth failure (`401`, `UNAUTHENTICATED`, `Unauthorized`) or when `session_status` shows no authenticated session.
- When auth fails, show the error and ask before triggering login.
- For Delio and Frisco network capture / remote debugging, use only port `9222`.
- Open the provider page in the default browser profile so existing logged-in apps are available.
- If port `9222` is not active, the CLI/MCP may close the current default browser instance and relaunch it on `9222`.
- After opening the page, keep it open for **3 seconds** before attempting to capture auth artifacts.
- Delio capture source: auth cookies (`authToken`, `idToken`, `refreshToken`) and related cookie header. Frisco capture source: access token / refresh token / user_id plus cookies when available.
- After capture, run a live verify against the provider.
- If verify fails, reload the page, wait **3 seconds**, then capture once more and verify again.
- There is **no fallback auth path** for this shopping flow. Do not switch to `session_from_curl` or another recovery method unless the user explicitly changes the policy. If this flow fails, shopping is blocked.

## Prerequisites

1. Run `session_status` for the chosen provider first.
2. If the user explicitly wants MCP, use MCP for product/cart work; use CLI only to diagnose or fix MCP, not to complete the shopping flow.
3. Only if `session_status` shows no authenticated session, or a real MCP/API request returns `401` / `UNAUTHENTICATED`, ask before `session_login` or `session_from_curl`.
4. If binary missing: `make build`.
5. Gather context from the user:
   - Dietary guidelines, restrictions, or just meal ideas.
   - How many people and which days (e.g., "weekend for 2").
   - Budget expectations (if any).
   - What they already have at home (oils, spices, staples) — ask early to avoid buying things they don't need.

## Phase 1: Meal Plan

- Build a plan matching the user's input (number of days, meals per day, portions).
- Save to a markdown file (not in the repo — ask user where they want it, e.g., Obsidian vault).
- The plan is a living document — update it whenever the cart changes.

## Phase 2: Product Search

Always use `--category-id` to narrow results. Without it, searches return irrelevant matches. Full category tree is in `categories.md` at the project root.

```bash
# Best-match pick (scores by name match + price/kg + pack size):
./bin/frisco products pick --search "PHRASE" --category-id CATID --top 3

# Raw search when pick isn't enough:
./bin/frisco products search --search "PHRASE" --category-id CATID --format json 2>/dev/null | \
  jq '[.products[] | select(.product.isAvailable == true) |
    {id: .productId, name: .product.name.pl, price: .product.price.price,
     grammage: .product.grammage, unit: .product.unitOfMeasure}] | .[0:5]'

# Add single product by search:
./bin/frisco cart add --search "PHRASE" --category-id CATID --quantity N
```

Use **subagents in parallel** to search multiple product groups at once (e.g., meat, dairy, vegetables, pantry). Each subagent uses the Go CLI + jq.

When a product is unavailable, search for alternatives and propose a substitution — update the meal plan accordingly.

## Phase 3: Cart Assembly

1. Collect all product IDs and quantities into a JSON file.
2. **Dry-run first**: `./bin/frisco cart add-batch --file FILE --dry-run`
3. Show the user a full summary table with names, prices, and quantities.
4. **Wait for explicit confirmation** before running without `--dry-run`.

## Phase 4: Nutrition / Macros

The user cares about nutritional values. For every meal plan, provide per-meal and per-day macro breakdowns (kcal, protein, fat, carbs).

**Data sources (in priority order):**

1. **Frisco API** — `./bin/frisco products nutrition --product-id PID --format json` returns per-100g values for many products. Use subagents to batch-query nutrition for all key ingredients.
2. **Standard nutritional databases** — for products where the API returns no data, use well-known reference values. Mark these clearly as "estimated" in the plan.

**How to present:**
- Add a macro summary table to each day in the plan.
- Add a weekly average at the bottom.
- Note which values come from the API vs estimates.
- Flag any days that seem too high or too low for the user's goals.

```bash
# Query nutrition for a product:
./bin/frisco products nutrition --product-id PID --format json 2>/dev/null | \
  jq '{kcal: (.nutrition.nutriments[] | select(.name.pl | test("kcal")) | .in100Gramms),
       protein: (.nutrition.nutriments[] | select(.name.pl | test("Białko")) | .in100Gramms),
       fat: (.nutrition.nutriments[] | select(.name.pl | test("^Tłuszcz")) | .in100Gramms),
       carbs: (.nutrition.nutriments[] | select(.name.pl | test("^Węglowodany")) | .in100Gramms)}'
```

## Phase 5: Cost Optimization

After the cart is filled, review it with the user:

```bash
# Most expensive items:
./bin/frisco cart show --sort-by total --top 20

# Or via JSON for detailed analysis:
./bin/frisco cart show --format json 2>/dev/null | \
  jq '[.products[] | select(.quantity > 0) |
    {name: (.product.name.pl // .product.name.en), price: .product.price.price,
     qty: .quantity, total: (.product.price.price * .quantity), grammage: .product.grammage}] |
  sort_by(-.total) | .[0:20]'
```

Propose savings by working through these strategies:

1. **Things the user already has** — pantry staples accumulate. Ask.
2. **Frozen vs fresh** — seasonal pricing matters. Frozen fruits/vegetables can be much cheaper.
3. **Waste reduction** — if a package is much larger than needed, either spread it across multiple days in the plan or find a smaller alternative. Fewer product types with full utilization beats variety that rots.
4. **Duplicates** — parallel searches can pick different products for the same ingredient.
5. **Quantity right-sizing** — don't order 2 packs when 1 covers the week.
6. **Cheaper alternatives** — different brands, non-BIO, different cuts, store brand.
7. **Better bought elsewhere** — some niche items are overpriced on Frisco vs pharmacies or specialty stores. Flag these.

After every change, show the updated cart total.

```bash
./bin/frisco cart remove --product-id PID
./bin/frisco cart add --product-id PID --quantity N
./bin/frisco cart remove-batch --product-ids PID1,PID2,PID3
```

## Phase 6: Plan Reconciliation

The meal plan must always reflect what's actually in the cart:

- **Substitutions**: update recipe text when swapping products.
- **Removed items**: rework affected meals to use remaining ingredients.
- **Portion changes**: recalculate when some days serve more people.
- **Ingredient budget**: for items with large packages, add a table showing which days use what amount, so nothing goes to waste.
- **Macros**: recalculate nutrition whenever ingredients change.

## Phase 7: Order Review

After the user places an order, review what was actually ordered:

```bash
# List recent orders:
./bin/frisco account orders list

# See products in a specific order:
./bin/frisco account orders products --order-id ORDER_ID --sort-by total

# Full order details:
./bin/frisco account orders get --order-id ORDER_ID
```

Use this to verify the order matches the plan and to inform future planning sessions.

## Principles

- **Respect the requested interface** — if the user asked for MCP, do not silently switch to CLI to finish the task. Use CLI only to debug/fix MCP unless the user explicitly allows otherwise.
- **Login only on real auth failure** — do not trigger `session_login` while `session_status` still reports an authenticated session unless an actual request fails with auth errors.
- **Delio and Frisco login flows differ** — Delio is cookie-based; Frisco requires access/refresh/user_id capture.
- **Verify after capture** — if the first verify fails, reload once, wait 3s, capture again, and re-verify once.
- **No fallback means blocked** — if the approved login flow fails, do not improvise alternate auth recovery; report that shopping is blocked.
- **Use the Go CLI when allowed** — `products pick`, `products search`, `cart add --search`, `cart show --sort-by`. No external scripts.
- **Confirm before cart changes** — always show what will happen and wait for OK.
- **Minimize waste** — adapt the plan to use what's bought in full.
- **Respect dietary restrictions** — never substitute excluded ingredients, even if cheaper.
- **Track nutrition** — the user wants to know what they're eating. Include macros in every plan.
- **Iterate** — present, discuss, adjust. Don't aim for perfection in one pass.
- **Show the running total** — the user should always know where they stand.
- **Keep personal files out of the repo** — plans, shopping lists, cart JSONs go where the user wants them (e.g., Obsidian vault), not in the git repo.
