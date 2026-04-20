# MartMart CLI

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
- **Backward-compatible local session storage**

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
| Checkout / payment finalization | ❌ | ❌ |

## Safety and scope

MartMart is intentionally focused on **non-finalizing** shopping workflows.

Implemented scope today:
- log in
- inspect products
- manage cart contents
- inspect delivery slots
- inspect account/order data where supported

Not implemented:
- final checkout
- payment confirmation
- placing final orders

That keeps the tool useful while reducing the risk of accidental purchases.

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

## Quick start

### 1) Log in with your browser profile

Recommended for both providers:

```bash
martmart session login
martmart --provider delio session login
```

MartMart opens Chrome/Chromium using a **temporary snapshot of your existing browser profile**.
If you are already logged in in your normal browser, the session is often captured automatically.
If not, log in in the opened window and wait for the CLI to save the session.

Useful flags:

```bash
martmart session login --profile-directory Default
martmart session login --user-data-dir "/path/to/browser/user-data"
martmart session login --timeout 240
```

### 2) Verify the session

```bash
martmart session verify
martmart --provider delio session verify
```

### 3) Start using it

#### Frisco examples

```bash
martmart products search --search banana
martmart cart show
martmart cart add --search "mleko" --quantity 1
martmart reservation slots --days 2
martmart account orders list --all-pages
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

```bash
martmart --provider frisco cart show
martmart --provider delio cart show
```

### Save default provider and rate limit

```bash
martmart config show
martmart config set --default-provider delio --rate-limit-rps 2 --rate-limit-burst 2
```

### Use JSON output for scripting

```bash
martmart cart show --format json
martmart products search --search mleko --format json
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
martmart cart show
martmart cart add --product-id <id> --quantity 1
martmart cart remove --product-id <id>
martmart products search --search <phrase>
martmart products get --product-id <id>
martmart reservation slots --days 2
martmart session login
martmart session verify
martmart mcp
```

## Batch shopping list input

Frisco supports batch additions from JSON shopping lists.

Example:

```bash
martmart cart add-batch --file list.json
```

Template:
- [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json)

## MCP integration

MartMart can run as an MCP server for AI clients:

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

## Local data layout

For backward compatibility, session/config files are still stored under `~/.frisco-cli/`.

- Frisco session: `~/.frisco-cli/session.json`
- Delio session: `~/.frisco-cli/delio-session.json`
- Shared config: `~/.frisco-cli/config.json`

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
- safer checkout research without finalizing purchases
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
