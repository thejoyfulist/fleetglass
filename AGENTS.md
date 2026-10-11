# Fleetglass — rules for agents working in this repository

Read `docs/PRODUCT.md` first: it is the source of truth for scope and principles.

- Code, comments, commit messages, and docs in this repository: English.
- Go: standard library first; any third-party module must be justified in the PR/report (license must be compatible with
  AGPL-3.0). `gofmt`, `go vet` clean.
- Web: React + TypeScript (Vite). No UI kit; design follows `docs/PRODUCT.md` (monochrome terminal cockpit: text tables, block bars, cyan warnings, magenta critical).
- Security: the agent never runs as root; privileged reads only through `helper/` and its fixed allowlist. Never log or
  display secrets, tokens, serial numbers or MAC addresses. Read-only: never change the host.
- Tests: behaviour-level tests with fixtures (fake /proc, /sys, journald output, Docker socket). Use temporary
  directories and unused loopback ports for integration and browser tests.
- Keep host-specific deployment decisions and credentials out of this repository.
- Documentation ships with the change, in the same commit: every user-visible change adds an entry under `## [Unreleased]` in
  `docs/CHANGELOG.md`, updates `README.md` when features, install steps, configuration or screenshots change, and
  updates the matching page in `docs/`. New config keys go in `docs/CONFIGURATION.md`. Screenshots use synthetic data only.
