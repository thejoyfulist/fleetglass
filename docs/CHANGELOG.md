# Changelog

All notable user-facing changes are documented here. This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and uses semantic release tags.

## [Unreleased]

### Added

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
