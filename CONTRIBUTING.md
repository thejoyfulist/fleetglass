# Contributing

Thanks for helping improve Fleetglass. Open an issue for larger changes before implementing them, then send a focused pull request.

## Build and verify

Use Go 1.26 and Node 24. The committed `web/dist` is embedded by the Go server.

```sh
go build ./...
go vet ./...
go test ./...
cd web
npm ci
npm test
npm run check-build
```

For UI edits, run `npm run build` in `web/` and commit the regenerated `web/dist` files. Run `python3 web/scripts/showcase.py` to regenerate screenshots. Browser checks use `python3 web/scripts/m10_terminal_visual.py`. Installer checks use `sh scripts/test-installer.sh`; release bundle tests use `scripts/test-install.sh` with a locally built archive.

## Data and changes

Use only synthetic fixtures (`demo-01`, `atlas`, `orbit`, `ledger`) in screenshots, docs, and tests. Do not commit real host data, account identifiers, secrets, serial numbers, or MAC addresses. The agent is read-only and never runs as root; changes to privileged reads belong in the helper's fixed allowlist.

Use short imperative commit subjects, such as `docs: clarify agent setup` or `fix: label unavailable quota`. Describe behavior, verification, and limitations in your pull request. New third-party Go modules need a clear reason and an AGPL-3.0-compatible license.
