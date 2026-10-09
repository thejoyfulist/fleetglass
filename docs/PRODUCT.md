# Fleetglass — product definition

Status: maintained product definition. Name: Fleetglass.
Source of truth for scope and principles.

## One-line pitch
The cockpit for servers that run AI coding agents: self-hosted, it ties every Claude Code / Codex session to the
machine it runs on (processes, CPU, memory, quotas, cost per project, incidents), next to complete server monitoring,
with no data leaving your machine.

Positioning (from `docs/COMPETITION.md`, 2026-10-08): lead with the agent-to-machine correlation nobody offers; do not
pitch "zero blind spots" head-on against Datadog or Netdata. Coverage stays complete, but it is not the headline.

## Target users
1. Primary: small teams and solo builders running AI coding agents (Claude Code, Codex CLI) on their own servers.
2. Secondary: homelab and indie hosting (Docker, VPS, mini-PC).
Out of scope: data centers, enterprise IT.

Differentiator: no other tool watches the machine *and* the agents running on it (sessions, tokens, quotas, cost per
project) together.

## Delivery and business model
- Decision 2026-10-08: no commercialisation for now. Goal: reach "reference" product quality first, released as open
  source. A paid offer (hosted SaaS, support) may be considered later, not designed now.
- Self-hosted: a lightweight agent plus a dashboard server on the user's machine. The agent is designed so a hosted
  mode could reuse it later.
- License: AGPL-3.0 for the whole product (multi-server and alerting included). Can still be revisited before the
  repository goes public.
- No telemetry, ever by default. Only an update check, which can be disabled.
- Name: Fleetglass. Trademark and domain decisions belong to the maintainer.

## Principles
- Zero blind spots: every domain below is covered in v1, except full-text log search (v1 shows recent errors only).
- Least privilege: the agent runs as a dedicated non-root user with precise read grants; privileged reads (SMART, etc.)
  go through a tiny separate helper with a fixed allowlist.
- Live: the dashboard updates in place (target: 5 to 15 s), never a full page reload.
- Read-only: Fleetglass observes; it never restarts, deploys or changes anything on the host.
- Honest numbers: every figure says what it measures (e.g. "CPU package power", not "wall power"); unknown is shown as
  unknown, never as healthy.
- Design (current direction): professional SaaS look, bento
  grid in strict rows (every tile aligned, no holes), frosted-glass tiles over a restrained slate background, one
  brand gradient (blue `#2F5BEA` to teal `#13A5B8`), Geist and Geist Mono. Light and dark themes, chosen by the user
  (first visit follows the system). Reference: `docs/design/cockpit-reference.html`; brand: `assets/brand/BRAND.md`
  (mark "Glass panes"). Semantic colour (amber, red) only on what deviates from normal; no emojis.
- No LLM in the monitoring path. Insights ("What stands out") come from fixed, documented rules.

## v1 coverage
1. Machine: CPU, memory, swap, disk and inodes, network, load, temperatures, fans, power (RAPL), disk health (SMART).
2. Services: systemd units, Docker (rootful and rootless), containers and health, listening ports, TLS certificates.
3. Uptime: internal probes, external HTTP probes, a minimal public status page (polish later).
4. Logs: recent errors only (journald priority ≤ err, container restarts, OOM). Full-text search: v2.
5. AI agents: running sessions (Claude Code, Codex CLI) with project, worktree and Unix user, tied to their process
   tree (CPU, memory); tokens per day and per project; quotas with pace (Codex `rate_limits`, Claude estimated);
   agent health (waiting for input, rate-limited, retries exhausted, orphan worktrees); multi-user servers.
   Ingestion: an OpenTelemetry (OTLP) receiver as the primary path (both CLIs export OTel), local session-log parsing
   as the fallback. Optional real cost via Anthropic / OpenAI billing APIs; cost per project combining subscriptions
   and API usage.
6. Security: pending updates, reboot required, open ports, SSH logins, sudo usage, failed logins, sensitive file changes.
7. Backups: age, size, off-site copy, last restore test.
8. Changes: package upgrades, deployments, service and machine restarts, git merges.
9. Alerts: mail, Telegram, Slack, Discord, generic webhook, ntfy, Healthchecks.io; one alert per state change with
   reminders while it lasts; acknowledge, snooze, maintenance windows; heartbeat / cron checks; an alert when the
   Fleetglass agent itself goes silent. PagerDuty and SMS: v2.
10. Incidents: timeline, duration, MTTR, 30-day availability.
11. Costs: energy, AI subscriptions and API usage. Cloud bills: v2.
12. Cross-cutting: change markers on every chart, command palette, Prometheus `/metrics` export. Later: optional
    read-only MCP endpoint.

Users in v1: one admin account, read-only share links, public status page. Teams, roles, SSO: v2.

## Architecture (v1)
- `agent/`: Go, single static binary, ~10 MB RAM target, runs as user `fleetglass`, collects and pushes to the server
  over HTTPS (or a local socket when co-located). Probes are isolated with timeouts; one failing probe never stops the
  others.
- `helper/`: Go, tiny root helper exposing a fixed allowlist of privileged reads over a local socket.
- `server/`: Go, HTTP API + SQLite storage with downsampling, embeds the web UI, serves live updates (SSE).
- `web/`: React + TypeScript (Vite), built once and embedded in the server binary. Charts drawn in-house (SVG/Canvas),
  no heavy charting dependency unless justified.
- Later (SaaS): Postgres and a time-series store; the same agent pushes to the cloud.

## Data retention defaults
Raw samples 7 days, hourly aggregates 90 days, daily aggregates 2 years, incidents and changes 1 year. Configurable.

## Milestones (reordered 2026-10-08 after `docs/COMPETITION.md`: the differentiator comes first)
1. Agent + server + storage for machine and services, single server.
2. AI agents: OTLP receiver + log fallback, sessions tied to processes, tokens, quotas with pace, agent health, costs.
3. Cockpit dashboard (`docs/design/cockpit-reference.html` made real and live), light and dark themes, agents from day one.
4. Alerts and incidents (with ack, snooze, maintenance windows, heartbeat checks, self-monitoring).
5. Multi-server and fleet view.
6. External probes and minimal public status page.
7. Security, backups (including restore tests), changes, energy and AI costs.
8. One-command install, documentation, project site, public open-source release.
9. The name behind every figure, from the Vitals teardown (`docs/COMPETITION-vitals.md`): apps grouped by unit,
   container and agent session, subsystem views, projects and dev servers, insights, storage map, settings,
   explanations, share card, read-only MCP endpoint. Planned before milestone 8.

Each milestone should be tested on an isolated host before release.

## Open items
- Any later paid offer is a maintainer decision. If one comes: free up to 3 servers with the
  agent features included; never price by tokens or metrics (see `docs/COMPETITION.md`).

## Ideas (not scheduled)
- Software spend from invoices (proposed in 2026-10, working name "Spend OS"). A growing team pays for
  dozens of SaaS tools and nobody holds the full list: trials silently turn paid, licences stay active after someone
  leaves, annual renewals land without warning. Every invoice already arrives by email. Fleetglass would connect to a
  mailbox (Gmail or Outlook, read-only), extract software invoices and keep an inventory: each subscription, its
  price, billing period and next renewal; upcoming renewals and tools that look unused are flagged ahead of time. No
  bank card or accounting tool to connect. Natural home: the Costs domain, next to AI subscriptions, so the user sees
  servers, agents and every software subscription in one place. Open questions before scheduling: mailbox access
  stays local and read-only with only extracted fields stored (never mail bodies); extraction without an LLM in the
  path, or an explicit opt-in exception; "unused" needs a usage signal beyond invoices; overlap with Zylo, Cledara,
  Vendr, Productiv.
