# Changelog

All notable user-facing changes are documented here. This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and uses semantic release tags.

## [Unreleased]

### Added

- A read-only cockpit for Linux server health, services, containers, projects, uptime, backups, security observations, and recent changes.
- Claude Code and Codex session attribution to processes and projects, with token pace, quota indicators, and labelled cost estimates.
- Multi-host enrollment, a fleet view, a minimal public status page, a read-only MCP endpoint, and Prometheus metrics.
- Light and dark terminal themes, keyboard shortcuts, English and French, a mobile layout, and an offline shell.
- Verified Linux amd64/arm64 release bundles, an installer, and online SQLite backups.

### Changed

- Session labels, host facts, CPU frequency, DNS probes, and cost by model are easier to inspect in the cockpit.

### Known limitations

- Alert delivery and incident acknowledgement are planned; current issues are visible in the cockpit but do not send notifications.
