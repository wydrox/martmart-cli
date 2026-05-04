# MartMart Shopping Chat macOS

Ultra-light native SwiftUI shell for shopping with `pi` + local `martmart`.

## Build

```bash
./scripts/macos-shopping-chat-build.sh
```

or directly:

```bash
swift build --package-path macos-app -c release
```

## Run

```bash
macos-app/.build/release/MartMartShoppingChat
```

Self-check without opening the UI:

```bash
macos-app/.build/release/MartMartShoppingChat --self-check
```

## MartMart binary discovery

`MartMartClient` looks for:

1. `./martmart` from the app launch working directory
2. `../martmart`
3. `martmart` on `PATH` via `/usr/bin/env`

For packaged local use, launch from the repo root or put `martmart` on `PATH`.

## Signing

The helper build script ad-hoc signs by default:

```bash
SIGN_IDENTITY=- ./scripts/macos-shopping-chat-build.sh
```

Use a Developer ID identity later if distributing outside local development.

## Permissions / privacy

MVP is intentionally unsandboxed for local development because it must:

- execute the local `martmart` helper
- read `~/.martmart-cli/catalog.db`
- let MartMart use saved provider sessions
- open browser/payment handoff URLs when checkout requires 3DS/redirect

If sandboxing is added later, define explicit entitlements for user-selected file access and replace direct helper execution with a signed bundled helper or XPC service.

## Safety

Checkout finalization is guarded:

- user must click **Zamów i zapłać**
- MartMart performs a fresh preview before finalize
- redirect/3DS returns an action handoff instead of pretending success
- logs/errors redact tokens/cookies/payment secrets
