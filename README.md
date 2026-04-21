# MartMart CLI

[![CI](https://github.com/wydrox/martmart-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/wydrox/martmart-cli/actions/workflows/ci.yml)
[![Release](https://github.com/wydrox/martmart-cli/actions/workflows/release.yml/badge.svg)](https://github.com/wydrox/martmart-cli/actions/workflows/release.yml)
[![Go 1.26+](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-compatible-7C3AED)](https://modelcontextprotocol.io)

**One grocery CLI for Frisco.pl and Delio.**

MartMart CLI gives you a single command-line interface for **session login**, **product search**, **cart operations**, **delivery slots**, and **MCP-based AI workflows** across supported grocery providers.

> **Status:** usable today for Frisco, MVP support for Delio.
>
> **Disclaimer:** independent community project, not affiliated with Frisco or Delio. Use only with **your own account/session** and at your own risk.

## Why MartMart CLI

MartMart started as a Frisco-focused CLI and evolved into a shared, provider-aware tool for grocery shopping workflows.

It is built for people who want to:
- automate shopping flows from the terminal
- reuse an existing browser session instead of copying headers manually every time
- inspect carts, products, and delivery options programmatically
- connect shopping workflows to AI clients through MCP
- keep one consistent CLI even when providers differ underneath

## Highlights

- **One CLI, multiple providers** via `--provider frisco|delio`
- **Unified browser-profile login** for Frisco and Delio
- **Product search and lookup**
- **Cart read/write operations**
- **Delivery slot lookup**
- **Frisco account and order commands**
- **MCP server** for Claude, Cursor, and other compatible clients
- **Configurable HTTP rate limiting**
- **Dedicated MartMart storage** in `~/.martmart-cli` with legacy read fallback from `~/.frisco-cli`

## Built for AI assistants and MCP

MartMart is not just a CLI. It can also run as a **Model Context Protocol (MCP)** server, making it useful for AI assistants that need structured access to grocery shopping workflows.

This repo is a good fit if you are looking for:
- a grocery shopping **MCP server**
- a **Claude Code** tool for Frisco.pl and Delio
- a **Cursor** or **Claude Desktop** integration for product search, cart operations, and delivery slot lookup
- a provider-aware shopping automation CLI in Go

MCP-oriented capabilities today:
- session login / verify tools
- product search and product details
- cart inspection and cart mutations
- delivery slot lookup
- Frisco account and order tools where supported

Start the MCP server:

```bash
martmart mcp
```

## Provider support

| Capability | Frisco | Delio |
|---|---:|---:|
| Browser-profile login | ✅ | ✅ |
| Session verify | ✅ | ✅ |
| cURL session import | ✅ | ✅ |
| Refresh token flow | ✅ | ❌ |
| Product search | ✅ | ✅ |
| Product details | ✅ | ✅ |
| Nutrition info | ✅ | ❌ |
| Cart show | ✅ | ✅ |
| Cart add/remove | ✅ | ✅ |
| Batch cart input | ✅ | ❌ |
| Interactive cart TUI | ✅ | ❌ |
| Delivery slots | ✅ | ✅ |
| Reservation reserve/cancel | ✅ | ❌ |
| Account / orders | ✅ | MVP / partial |
| MCP support | ✅ | partial, shared CLI path |
| Checkout / payment finalization | ⚠️ experimental: preview + guarded finalize (`--confirm`) | ⚠️ experimental: preview + guarded finalize (`--confirm`, Adyen-backed) |

## Safety and scope

MartMart stays **safe by default** and avoids implicit purchases.

Implemented scope today:
- log in
- inspect products
- manage cart contents
- inspect delivery slots
- inspect account/order data where supported
- preview Frisco checkout
- run **guarded** Frisco finalization only with explicit `checkout finalize --confirm`
- preview Delio checkout
- run **guarded** Delio finalization only with explicit `checkout finalize --confirm`

Still not implemented / not supported:
- automatic background finalization from MCP or agent loops
- redirect / 3DS completion inside the CLI after the external handoff step
- production-grade proof of final order placement for every Delio payment branch

Delio checkout is now wired into the user-facing CLI as an **experimental** flow. Under the hood it uses the captured Delio GraphQL + Adyen sequence for payment settings, payment creation, payment-method discovery, and `makePayment`, while still returning redirect / 3DS branches as structured external handoff data instead of trying to finish them inside the CLI.

That keeps the tool useful while still requiring an explicit opt-in before any final order submission.

Frisco checkout support is implemented as an **experimental guarded flow**. The remaining gap is live confirmation of the exact browser/API contract from full-session captures, especially for redirect / 3DS and negative cases.

## Installation

### Install with Go

```bash
go install github.com/wydrox/martmart-cli/cmd/martmart@latest
```

### Build locally

```bash
make build
./bin/martmart --help
```

### Dodanie do PATH (żeby `martmart` działało z każdego katalogu)

```bash
# tymczasowo (na sesję terminala):
export PATH="$PATH:$(pwd)/bin"

# trwale (zwykle ~/.zshrc dla macOS z Zsh):
echo 'export PATH="$PATH:$(pwd)/bin"' >> ~/.zshrc
source ~/.zshrc

# alternatywnie: link w ~/.local/bin
mkdir -p ~/.local/bin
ln -sf "$(pwd)/bin/martmart" ~/.local/bin/martmart
export PATH="$PATH:~/.local/bin"
```

## Quick start

### 1) Log in with your browser profile

Recommended for both providers:

```bash
martmart --provider frisco session login
martmart --provider delio session login
```

For **Delio on macOS**, MartMart opens the URL in your default browser and reads the needed session data from that browser profile.
If you are not logged in yet, sign in in the opened browser tab/window and wait for the CLI to save the session.

Useful flags:

```bash
martmart --provider frisco session login --profile-directory Default
martmart --provider frisco session login --user-data-dir "/path/to/browser/user-data"
martmart --provider frisco session login --timeout 240
martmart --provider frisco session login --debug --keep-open-on-failure
```

### 2) Verify the session

```bash
martmart --provider frisco session verify
martmart --provider delio session verify
```

Inspect stored sessions across providers when needed:

```bash
martmart session list
```

### 3) Start using it

#### Frisco examples

```bash
martmart --provider frisco products search --search banana
martmart --provider frisco cart show
martmart --provider frisco cart add --search "mleko" --quantity 1
martmart --provider frisco reservation slots --days 2
martmart --provider frisco account orders list --all-pages
```

#### Delio examples

```bash
martmart --provider delio products search --search mleko
martmart --provider delio products get --product-id A0000860
martmart --provider delio cart show
martmart --provider delio cart add --product-id A0000860 --quantity 1
martmart --provider delio reservation slots
```

## Common workflows

### Switch provider per command

Provider must be passed explicitly for provider-specific actions.

```bash
martmart --provider frisco cart show
martmart --provider delio cart show
```

### Save rate limit settings

```bash
martmart config show
martmart config set --rate-limit-rps 2 --rate-limit-burst 2
```

### Use JSON output for scripting

```bash
martmart --provider frisco cart show --format json
martmart --provider frisco products search --search mleko --format json
```

### Import session from cURL when needed

Frisco example:

```bash
martmart session from-curl --curl "curl 'https://www.frisco.pl/app/commerce/api/v1/users/123/cart' -H 'authorization: Bearer ...' -H 'cookie: ...'"
```

Delio example:

```bash
martmart --provider delio session from-curl --curl "curl 'https://delio.com.pl/api/proxy/delio' -H 'cookie: ...' -H 'x-platform: web' -H 'x-api-version: 4.0' -H 'x-app-version: 7.32.6' -H 'content-type: application/json' --data-raw '{...}'"
```

## Core commands

```bash
martmart --provider frisco cart show
martmart --provider frisco cart add --product-id <id> --quantity 1
martmart --provider frisco cart remove --product-id <id>
martmart --provider frisco products search --search <phrase>
martmart --provider frisco products get --product-id <id>
martmart --provider frisco reservation slots --days 2
martmart --provider frisco session login
martmart session list   # inspect stored sessions across providers
martmart --provider frisco session verify
martmart mcp
```

## Voice shopping assistant (Pipecat + OpenAI Realtime)

`martmart voice` starts a local voice assistant powered by one OpenAI model for speech-to-speech and connected to local MCP with `martmart mcp`.

Requirements:
- OpenAI key configured in CLI config (`martmart config set ...`).
- an authenticated MartMart session (`session login` / `session verify`)
- Python 3.12+ and working local audio (microphone/speaker)

### Konfiguracja klucza i ustawień (TUI)

Najwygodniej otworzyć TUI:

```bash
martmart config
```

albo skrótowo:

```bash
martmart config set
```

W TUI możesz edytować:
- domyślny provider,
- limity requestów,
- klucz OpenAI,
- model/voice/language/transcription model,
- prędkość głosu,
- wejście/wyjście audio.

Jeśli chcesz ustawić tylko jeden parametr bez TUI:

```bash
martmart config set --openai-api-key sk-proj-...
```

### Setup asystenta

```bash
martmart voice setup
```

Lub manualnie:

```bash
git clone ...
cd martmart-cli
python3 -m venv ~/.martmart-cli/voice/venv
source ~/.martmart-cli/voice/venv/bin/activate
pip install -r ~/.martmart-cli/voice/requirements.txt
```

### Uruchomienie

```bash
martmart voice run
```

Jeśli `OPENAI_API_KEY` nie jest ustawiony w systemie, zostanie użyty klucz z `config`.

Optional flags:

```bash
martmart voice run --model gpt-realtime --voice alloy --language pl --input-device -1 --output-device -1
```

Domyślne ustawienia modelu głosowego biorą się z `martmart config`:

```bash
martmart config show
```

Asystent automatycznie pyta o marki, promocje, zamienniki i produkty komplementarne, a po zmianie koszyka podsumowuje kroki i rekomendacje.

Możesz też wywołać:

```bash
martmart voice
```

co odpala ten sam runtime, co `martmart voice run`.

## Batch shopping list input

Frisco supports batch additions from JSON shopping lists.

Example:

```bash
martmart --provider frisco cart add-batch --file list.json
```

Template:
- [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json)

## Checkout flow fixtures and current CLI support

MartMart currently exposes **experimental checkout flows for Frisco and Delio** with preview and explicit guarded finalization.

Target flow:

1. build the cart
2. choose or reserve a delivery window
3. request a **checkout preview**
4. inspect totals / delivery / payment state
5. only with an explicit user action, run **finalize** with `--confirm`
6. if the provider responds with a hosted redirect / 3DS step, stop the CLI flow and hand the user the redirect details

Current limitations:

- finalization must never happen implicitly from MCP or agent loops
- the finalize command requires explicit `--confirm`
- redirect / 3DS is currently **handoff-only**: the CLI detects it, returns structured action data, and lets the user finish externally
- Delio currently uses the captured GraphQL + Adyen payment flow and may end in `pending` / `requires_action` instead of a proven placed order
- the exact live finalization contract still benefits from confirmation from real request/response captures

Frisco repo fixtures:

- [examples/checkout-preview.request.example.json](examples/checkout-preview.request.example.json)
- [examples/checkout-preview.response.example.json](examples/checkout-preview.response.example.json)
- [examples/checkout-finalize.request.example.json](examples/checkout-finalize.request.example.json)
- [examples/checkout-finalize.response.example.json](examples/checkout-finalize.response.example.json)
- [examples/checkout-finalize.redirect-3ds.response.example.json](examples/checkout-finalize.redirect-3ds.response.example.json)

Delio contract-reference fixtures:

- [examples/delio-checkout-preview.request.example.json](examples/delio-checkout-preview.request.example.json)
- [examples/delio-checkout-preview.response.example.json](examples/delio-checkout-preview.response.example.json)
- [examples/delio-checkout-finalize.request.example.json](examples/delio-checkout-finalize.request.example.json)
- [examples/delio-checkout-finalize.response.example.json](examples/delio-checkout-finalize.response.example.json)
- [examples/delio-checkout-finalize.redirect-3ds.response.example.json](examples/delio-checkout-finalize.redirect-3ds.response.example.json)

These examples are **redacted / provisional**. They document the Delio GraphQL + Adyen contract shapes reflected in the current implementation, while still acknowledging that some live browser-confirmed success / 3DS branches need more capture coverage.

For Frisco and Delio, MartMart returns a structured action handoff when finalize hits a redirect / 3DS stop. For Delio, that handoff is based on the captured Adyen-backed `makePayment` flow.

### Remaining evidence to harden live checkout finalization

To move from an experimental implementation to a confirmed production-grade flow, the repo still needs live browser/API evidence for the final Frisco step:

1. **Checkout preview request + response**
   - full request path/method
   - JSON body actually accepted by Frisco
   - success response fields used for totals, delivery slot, payment method, warnings, and order readiness
2. **Finalize request + success response**
   - exact endpoint/path
   - headers/cookies/auth requirements
   - request body including any confirmation/idempotency fields
   - response proving an order was placed (order id / status / receipt-like summary)
3. **Redirect / 3DS branch**
   - finalize response when card auth is required
   - fields for redirect URL, method, payload/form fields, transaction id, and return/callback identifiers
   - the follow-up request/response that turns a successful 3DS challenge into a completed order (or marks it failed)
4. **Negative cases**
   - expired reservation / slot conflict
   - price changed / product unavailable
   - payment method rejected
   - duplicate-submit / retry behavior

A HAR file or equivalent redacted request/response log covering those cases is the exact blocker-removal artifact.

## AI assistant and MCP integration

MartMart exposes a **stdio MCP server** for AI clients.
That means the same project can be used both as a human CLI and as a tool backend for agents.

Typical agent tasks:
- verify a shopping session
- search for products
- inspect a product
- show cart contents
- add or remove cart items
- inspect delivery slots
- access Frisco account/order data where supported

Start the MCP server directly:

```bash
martmart mcp
```

### Claude Code

```bash
claude mcp add martmart -- martmart mcp
```

### Claude Desktop

```json
{
  "mcpServers": {
    "martmart": {
      "command": "martmart",
      "args": ["mcp"]
    }
  }
}
```

### Cursor

```json
{
  "mcpServers": {
    "martmart": {
      "command": "martmart",
      "args": ["mcp"]
    }
  }
}
```

### Other MCP clients

Any client that supports stdio-based **Model Context Protocol** servers should work with:

```bash
martmart mcp
```

## Example agent prompts and MCP workflows

Example prompts you can give an MCP-capable assistant:

- "Check session_status for Frisco. If it is not authenticated, ask me and then log me in."
- "Use Delio, search for oat milk, and show the top 5 results as JSON."
- "Show my Frisco cart and summarize total quantity by item."
- "Add 1 carton of milk to my Delio cart and then show the updated cart."
- "Check delivery slots for the next 2 days on Frisco."
- "Search Frisco for spaghetti, parmesan, and pancetta, then propose a carbonara shopping list."
- "Prepare a Frisco checkout preview, but do not finalize anything unless I explicitly confirm."

Example safe MCP workflows:

1. **Session bootstrap**
   - choose the provider for that request
   - run `session_status`
   - only if needed and with user confirmation, run `session_login`
   - then run `session verify`
2. **Product discovery**
   - search products
   - inspect a specific product
   - compare multiple candidate items
3. **Cart update loop**
   - show current cart
   - add/remove items
   - show the resulting cart again
4. **Delivery planning**
   - fetch available slots
   - compare providers if needed
   - keep automatic checkout/finalization out of scope unless the user explicitly asks for guarded Frisco checkout
5. **Checkout preview / guarded finalize (Frisco only)**
   - reserve/confirm the intended delivery context first
   - run `checkout preview`
   - inspect totals, warnings, payment state, and whether redirect / 3DS might be required
   - call `checkout finalize --confirm` only with explicit user approval
   - if finalize returns a structured redirect / 3DS action, hand that URL/payload back to the user for external completion instead of trying to finish it inside the CLI

These flows are intentionally designed around **non-finalizing by default** behavior.

## Local data layout

Session/config files are stored under `~/.martmart-cli/`.

- Frisco session: `~/.martmart-cli/frisco-session.json`
- Delio session: `~/.martmart-cli/delio-session.json`
- Shared config: `~/.martmart-cli/config.json`
- Use `martmart session list` to inspect stored sessions across all providers at once.

Planned local catalog work:
- product catalog ingest plan: [`docs/product-catalog-ingest-plan.md`](docs/product-catalog-ingest-plan.md)

Legacy compatibility:
- if a file is missing in `~/.martmart-cli/`, MartMart will also try older Frisco session locations such as `~/.martmart-cli/session.json` and `~/.frisco-cli/session.json`
- new saves go to `~/.martmart-cli/`

## Security notes

- session files may contain cookies, tokens, and reusable auth headers
- anyone with access to those files may act in your account context
- never share session files publicly
- do not run `martmart mcp` in untrusted environments
- use only your own browser session and your own account

## Roadmap

Near-term improvements:
- stronger MCP/provider parity beyond the Frisco-first surface
- better provider-aware account/order coverage
- improved browser/profile discovery for login
- confirm and harden the live Frisco checkout endpoint and success/error contract from full-session captures
- add richer redirect / 3DS continuation support after the external handoff step
- better docs and examples for automation flows

## Development

```bash
make setup
make build
make test
```

Direct commands:

```bash
go test ./...
go vet ./...
go build ./cmd/martmart
```

## License

[MIT](LICENSE)
