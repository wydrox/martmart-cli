# Contributing

Thanks for contributing to `martmart`.

## Local setup

Requirements:

- Go version from `go.mod`
- [golangci-lint](https://golangci-lint.run/welcome/install/) (recommended)

```bash
make setup   # configure pre-commit hook (once)
make build
./bin/martmart --help
```

## Quality checks

The pre-commit hook runs `golangci-lint` and `go test` before each commit. CI also enforces `gofmt`, `go vet`, `govulncheck`, and `go build`.

Before opening a PR, run (or rely on `make lint` / `make test` where noted):

```bash
# formatting (CI: fail if gofmt would change tracked .go files)
out=$(git ls-files '*.go' | xargs gofmt -l)
if [ -n "$out" ]; then echo "$out"; exit 1; fi

make lint    # golangci-lint (includes many analyzers; does not replace the gofmt check above)

# vulnerability scan (install once: go install golang.org/x/vuln/cmd/govulncheck@latest)
govulncheck ./...

make test    # go test ./... (CI uses go test -race)
go vet ./...
go build ./cmd/martmart
```

If the `gofmt` step fails, run `go fmt ./...` and commit again.

## Security and secrets

- Never commit local session files or credentials.
- Do not include real tokens/cookies in examples, logs, or test fixtures.
- Security reports should follow `SECURITY.md`.

## Pull requests

- Keep PRs focused and small when possible.
- Update `README.md` if CLI flags/behavior change.
- Add or update tests for behavior changes.
