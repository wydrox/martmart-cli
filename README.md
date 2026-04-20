# MartMart CLI

Unofficial shared CLI for the [Frisco.pl](https://www.frisco.pl) and [Delio](https://delio.com.pl) grocery delivery APIs.

> **Disclaimer**: This is an independent, community project — not affiliated with Frisco. Use at your own risk.

## Features

- Shared provider-aware CLI (`--provider frisco|delio`)
- Interactive cart TUI (Bubble Tea) for Frisco
- Product search with category filtering and nutrition info
- Delio MVP: session verify, product search/get, cart show/add/remove, delivery slots
- Delivery slot reservation
- Order history
- Batch cart additions from JSON shopping lists
- MCP server for AI assistant integration
- Session management (cURL import, browser-profile session capture for Frisco and Delio)
- Configurable request rate limiter


## Requirements

- Go `1.26+` (per `go.mod`)
- For `martmart session login`: a locally installed browser supported by `chromedp` (e.g. Chrome/Chromium)
- `session login` works best when you are already logged in in your normal Chrome/Chromium profile

## Installation

```bash
go install github.com/wydrox/martmart-cli/cmd/martmart@latest
```

Local build:

```bash
make build
./bin/martmart --help
```

## Quick start

1. Import a session from your browser profile (recommended):

```bash
martmart session login
martmart --provider delio session login
```

This opens Chrome/Chromium using a temporary snapshot of your current browser profile. If you are already logged in in your normal browser, the CLI should capture the session automatically. Otherwise, sign in in the opened window and wait for the session to be saved.

2. Or save session from a cURL command:

```bash
martmart session from-curl --curl "curl 'https://www.frisco.pl/app/commerce/api/v1/users/123/cart' -H 'authorization: Bearer ...' -H 'cookie: ...'"
```

3. Verify your session works:

```bash
martmart session verify
```

4. Optional: switch provider per command or save a default:

```bash
martmart --provider delio session from-curl --curl "curl 'https://delio.com.pl/api/proxy/delio' -H 'cookie: ...' -H 'x-platform: web' -H 'x-api-version: 4.0' -H 'x-app-version: 7.32.6' -H 'content-type: application/json' --data-raw '{...}'"
martmart config set --default-provider delio --rate-limit-rps 2 --rate-limit-burst 2
```

## Commands

```bash
martmart cart                    # Interactive TUI (Frisco only)
martmart cart show
martmart cart add --product-id <id> --quantity 1
martmart cart add-batch --file list.json        # --dry-run for parse-only
martmart products search --search banana
martmart products search --search apple --category-id 18707
martmart products get --product-id 4094
martmart products nutrition --product-id 4094
martmart reservation slots --days 2
martmart reservation reserve --date 2026-03-25 --from-time 06:00 --to-time 07:00
martmart reservation cancel
martmart account show
martmart account orders list --all-pages
martmart session login               # Browser-profile session capture (Frisco)
martmart --provider delio session login
martmart session refresh-token       # Refresh access token (Frisco)
martmart config show
martmart config set --default-provider delio --rate-limit-rps 2 --rate-limit-burst 2
martmart --provider delio products search --search mleko
martmart --provider delio products get --product-id A0000860
martmart --provider delio cart show
martmart --provider delio reservation slots
martmart mcp                         # Start MCP server
```

### Batch cart from a shopping list

1. `martmart session verify` — make sure your session is valid.
2. Search for products: `martmart products search --search "phrase"` — optionally add `--category-id` to narrow results. Use `--format json` and pipe to `jq` to extract product IDs.
3. Build a JSON file: an array of `{ "product_id": "…", "quantity": n }` objects or `{"items":[…]}`. Template: [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json).
4. `martmart cart add-batch --file list.json` — displays a cart summary after adding. Use `--dry-run` to validate the file without calling the API.

## Category IDs

Using `--category-id` with `products search` / `products pick` narrows results and dramatically improves relevance. Full category tree: [categories.md](categories.md).

## Output format

By default the CLI outputs human-readable tables. Use `--format json` for machine-readable output:

```bash
martmart account orders list --all-pages
martmart account orders list --all-pages --format json
```

## MCP server — AI assistant integration

`martmart mcp` starts an [MCP](https://modelcontextprotocol.io) server over stdio, exposing cart, products, orders, reservations, and session tools to any compatible AI client.

### Claude Desktop

Download the latest `.mcpb` bundle for your platform from [Releases](https://github.com/wydrox/martmart-cli/releases), then in Claude Desktop go to **Extensions → Advanced Settings → Install Extensions** and select the downloaded file.

**Login:** Ask Claude to log you in to Frisco (e.g. "zaloguj mnie do Frisco"). A Chrome window will open — log in on frisco.pl and navigate to your cart or account page. The session is captured automatically and saved for future use. Requires Chrome/Chromium.

<details>
<summary>Manual setup</summary>

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

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

If `martmart` is not in your `PATH`, use the absolute path to the binary (e.g. `"/Users/you/go/bin/martmart"`).

</details>

### Cursor

[![Install MCP Server](https://cursor.com/deeplink/mcp-install-dark.svg)](https://cursor.com/install-mcp?name=martmart&config=eyJjb21tYW5kIjoibWFydG1hcnQiLCJhcmdzIjpbIm1jcCJdfQ==)

<details>
<summary>Manual setup</summary>

Add to `.cursor/mcp.json`:

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

</details>

### Claude Code

```bash
claude mcp add martmart -- martmart mcp
```

<details>
<summary>Other MCP clients</summary>

Any client supporting stdio transport works:

```bash
martmart mcp
```

The server exposes tools for: product search, cart management, order history, delivery reservation, account info, and session management.

</details>

## Session data

Session is stored locally under `~/.frisco-cli/`.

- Frisco keeps the legacy path: `~/.frisco-cli/session.json`
- Delio uses: `~/.frisco-cli/delio-session.json`
- Shared config is stored in: `~/.frisco-cli/config.json`

`session login` launches Chrome/Chromium against a temporary snapshot of your existing browser profile, so the CLI can reuse your logged-in browser session without modifying your main profile.

Security notes:

- The file may contain tokens and session headers (`Authorization`, `Cookie`).
- Access to this file grants API access in your account's context.
- Do not run `martmart mcp` in untrusted or shared environments.

If an access token expires, the CLI will automatically attempt to refresh it. If refresh fails, re-import your session with `martmart session from-curl` or `martmart session login`.

## Example payloads

- [examples/shipping-address.example.json](examples/shipping-address.example.json)
- [examples/reservation-payload.example.json](examples/reservation-payload.example.json)
- [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json)

## License

[MIT](LICENSE)
