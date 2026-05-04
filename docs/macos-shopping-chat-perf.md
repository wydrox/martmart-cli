# macOS Shopping Chat performance notes

Current app dependencies are intentionally minimal:

- SwiftUI / Foundation / Observation
- system SQLite (`libsqlite3`)
- no Electron
- no web app shell
- no third-party UI framework

Measured via:

```bash
./scripts/macos-shopping-chat-perf.sh
```

Latest local release build measurement:

- release binary: ~1.3 MB
- `--self-check` cold process: ~0.34 s real
- max resident set in self-check: ~9 MB reported by `/usr/bin/time -l`
- peak memory footprint: ~2 MB reported by `/usr/bin/time -l`

MartMart first search measurement attempted, but current Frisco session returned `401 Unauthorized`; the timing path still completed in ~0.54 s for the failed request. Re-run after refreshing provider session for a valid live-search number.
