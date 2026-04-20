# MartMart CLI

MartMart CLI is a unified command-line client for grocery shopping flows across **Frisco.pl** and **Delio**.

It gives you one interface for session handling, product discovery, cart operations, delivery slots, and MCP-based AI integration.

> Disclaimer: this is an independent community project. It is not affiliated with Frisco or Delio. Use at your own risk and only with your own account/session.

## What MartMart CLI does

- one CLI for multiple grocery providers
- unified browser-profile login flow for Frisco and Delio
- product search and product lookup
- cart inspection and cart mutations
- delivery slot lookup
- Frisco account and order operations
- MCP server for AI clients
- configurable request rate limiting

## Provider support

### Frisco

Implemented:
- session login / verify / from-curl / refresh-token
- products search / get / nutrition / pick
- cart show / add / remove / add-batch / remove-batch / TUI
- reservation slots / reserve / cancel / planning flows
- account and orders commands
- MCP support

### Delio

Implemented MVP:
- session login / verify / from-curl
- products search / get
- cart show / add / remove
- delivery slot lookup

Not implemented yet:
- checkout / payment finalization
- full account/order coverage
- refresh-token flow

## Installation

### Go install

```bash
go install github.com/wydrox/martmart-cli/cmd/martmart@latest
```

### Local build

```bash
make build
./bin/martmart --help
```

## Quick start

### 1. Log in with your browser profile

Recommended flow for both providers:

```bash
martmart session login
martmart --provider delio session login
```

This opens Chrome/Chromium using a **temporary snapshot of your existing browser profile**.
If you are already logged in in your normal browser, MartMart will usually capture the session automatically.
If not, log in in the opened window and wait for the CLI to save the session.

Optional flags:

```bash
martmart session login --profile-directory Default
martmart session login --user-data-dir "/path/to/browser/user-data"
martmart session login --timeout 240
```

### 2. Verify the session

```bash
martmart session verify
martmart --provider delio session verify
```

### 3. Start using the CLI

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

## Using a specific provider

By default the provider comes from saved config. You can override it per command:

```bash
martmart --provider frisco cart show
martmart --provider delio cart show
```

Supported values:
- `frisco`
- `delio`

## Config

Show current config:

```bash
martmart config show
```

Save default provider and rate limits:

```bash
martmart config set --default-provider delio --rate-limit-rps 2 --rate-limit-burst 2
```

Global flags:
- `--provider`
- `--format table|json`
- `--rate-limit-rps`
- `--rate-limit-burst`

## Session import from cURL

If needed, you can still import a session from a copied browser request.

### Frisco example

```bash
martmart session from-curl --curl "curl 'https://www.frisco.pl/app/commerce/api/v1/users/123/cart' -H 'authorization: Bearer ...' -H 'cookie: ...'"
```

### Delio example

```bash
martmart --provider delio session from-curl --curl "curl 'https://delio.com.pl/api/proxy/delio' -H 'cookie: ...' -H 'x-platform: web' -H 'x-api-version: 4.0' -H 'x-app-version: 7.32.6' -H 'content-type: application/json' --data-raw '{...}'"
```

## Common commands

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

## Batch cart input

MartMart supports batch additions from JSON shopping lists.

Example flow:
1. verify session
2. search product IDs
3. prepare JSON file
4. run:

```bash
martmart cart add-batch --file list.json
```

Template:
- [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json)

## Output modes

Default output is human-readable.
Use JSON for scripting:

```bash
martmart cart show --format json
martmart products search --search mleko --format json
```

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

Project-level `.cursor/mcp.json`:

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

## Session storage

Session data is currently stored under `~/.frisco-cli/` for backward compatibility.

Files:
- Frisco: `~/.frisco-cli/session.json`
- Delio: `~/.frisco-cli/delio-session.json`
- Shared config: `~/.frisco-cli/config.json`

## Security notes

- session files may contain tokens and cookies
- anyone with access to those files may act in your account context
- do not share session files
- do not run `martmart mcp` in untrusted environments
- use only your own browser session and account

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
