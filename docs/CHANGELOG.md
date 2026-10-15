# Changelog

All notable user-facing changes are documented here. This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and uses semantic release tags.

## [Unreleased]

### Fixed

- Local MCP and tokenless OTLP reject non-loopback HTTP Host headers; privileged SMART output is bounded to 1 MiB.
- Cockpit/API requests now enforce an explicit HTTP Host allowlist and same-host browser Origin; reverse proxies must declare their hostname in `allowed_hosts`.
- Go builds now require 1.26.9, which includes the current standard-library security fixes.

- Possible-loop warnings now use the observed tracking window, require recent token output, and recognize changes in attributed worktrees from cached Git scans.

### Added

- Optional Claude Code status-line bridge for reported plan limits, with safe chaining of an existing status command and local OTLP delivery.

- Per-session last file edit and last tool timestamps, a configurable possible-loop warning, and an edit-age column in the cockpit and read-only MCP output.

- Per-metric provenance for agent sessions: the session view shows the feed beside each value (`otel` OpenTelemetry, `log` local logs, `proc` process attribution, `est.` estimated), and tables tag each session `otel`, `log`, `otel+log` or `proc`.
- A `~` prefix on every estimated value, including API-equivalent cost.

### Changed

- GitHub Actions are pinned to commit SHAs and Dependabot watches actions, Go modules and npm packages.

## [0.9.0] - 2026-10-12

First public release.

### Added

- A read-only cockpit for Linux server health, services, containers, projects, uptime, backups, security observations, and recent changes.
- Claude Code and Codex session attribution to processes and projects, with token pace, quota indicators, and labelled cost estimates.
- Multi-host enrollment, a fleet view, a minimal public status page, a read-only MCP endpoint, and Prometheus metrics.
- Light and dark terminal themes, keyboard shortcuts, English and French, a mobile layout, and an offline shell.
- Verified Linux amd64/arm64 release bundles, an installer, and online SQLite backups.
- Readable session labels, host facts, CPU frequency, DNS probes, and cost by model.
- A one-line installer that verifies the release checksum before installing.

### Known limitations

- Alert delivery and incident acknowledgement are planned; current issues are visible in the cockpit but do not send notifications.
