# Fleetglass — product definition

Fleetglass is a self-hosted, read-only cockpit for Linux servers that run AI coding agents. It connects Claude Code and Codex sessions to their processes, projects, CPU, memory, token use, quotas, and labelled cost estimates beside the health of the machine they run on.

## Who it is for

Small teams, solo builders, and homelab operators running their own Linux servers. Fleetglass focuses on one question that general server monitors leave open: which agent session is using the machine right now, and what is it doing to cost and capacity?

## Principles

- **Read-only:** Fleetglass observes. It never stops an agent, restarts a service, deploys code, or changes host state.
- **Least privilege:** the agent runs as a dedicated non-root user. Privileged reads go through a small helper with a fixed allowlist.
- **Local by default:** cockpit and OTLP listeners bind to loopback. No product telemetry is sent to the maintainers; the optional update check is off by default.
- **Honest measurements:** figures say what they measure. Unknown values stay unknown; cost estimates are labelled by method.
- **Live cockpit:** the dashboard updates in place and supports keyboard navigation, light and dark themes, English and French, and narrow screens.
- **Deterministic insight:** fixed, documented rules explain what stands out. No LLM is in the monitoring path.

## Current scope

- Machine: CPU, memory, swap, disk and inodes, network, load, temperatures, fans, measured RAPL package power, and SMART health where access is available.
- Services: selected systemd units, Docker containers and health, listening ports, and TLS certificates.
- Availability: internal and external probes, history, and a minimal public status page.
- AI agents: Claude Code and Codex OTLP ingestion; optional local log fallback; sessions tied to process trees, projects, users, and worktrees; token pace, quota indicators, and cost estimates.
- Projects and attribution: dev servers, ports, worktrees, storage usage, app groups, and fixed-rule insights.
- Operations: pending updates, recent journal errors, backups, restarts and changes, and a read-only Prometheus endpoint.
- Fleet: multiple hosts, remote agent enrollment, and buffered delivery.
- Integration: an optional token-protected, loopback-only, read-only MCP endpoint with four tools.

Coverage depends on host permissions, hardware, and configured roots. Recent errors are shown; full-text log search is outside the current scope. See [configuration](CONFIGURATION.md) and [agent setup](AI-AGENTS.md) for the exact inputs.

## Design

The cockpit uses a monochrome terminal palette in light and dark themes. Neutral text and surfaces carry the data; cyan marks warnings and magenta marks critical states. Tables, block bars, and square panels keep measurements legible. Figures should show source, units, and uncertainty when relevant.

## Architecture

- `agent/`: an unprivileged Go collector with timeouts and isolated probes.
- `helper/`: a small root service exposing only fixed privileged reads over a local socket.
- `server/`: an HTTP API and SQLite store with downsampling and live SSE updates.
- `web/`: a React/TypeScript cockpit embedded in the server binary.

The default retention policy keeps raw samples for 7 days, hourly aggregates for 90 days, daily aggregates for 2 years, and incidents and changes for 1 year. It is configurable.

## Planned

Alert delivery and incident workflows, including acknowledgement, snooze, and maintenance windows, are on the roadmap. Further agent integrations may follow. The current cockpit displays observed issues but does not send notifications. The [changelog](CHANGELOG.md) records shipped user-facing changes.

Fleetglass is licensed under [AGPL-3.0](../LICENSE).
