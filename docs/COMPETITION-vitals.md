# Vitals (vitalsmac.com) — functional teardown and what Fleetglass takes from it

Studied 2026-10-09 from public material only (home page, `llms.txt`, guides index, roadmap, changelog 1.4).
Vitals is a closed-source macOS app (Fore Studio, $39 one-time); nothing was decompiled or copied. This page maps
its user-facing behaviour to a Linux server that runs AI coding agents, and records what Fleetglass adopts.

## What Vitals does, and how it lands for Fleetglass

| Vitals | What it means on a Fleetglass server | Decision |
|---|---|---|
| Processes grouped by app ("60 apps instead of 1,000 processes"), one figure each for memory, CPU, energy, disk, network | Group by systemd unit, container and agent session (cgroup v2), with CPU, memory, disk I/O, estimated energy share; network where measurable (containers) | **Adopt** (M9a) |
| A tab per subsystem with the apps behind each figure: Overview, CPU, Memory, Disk, Network, GPU, Containers | CPU (user/system/iowait/steal, load, per core), Memory (used, cache, buffers, swap, zram, pressure PSI), Disk (per volume, read/write speed, written today), Network (per interface, today / 7 d / 30 d), GPU when present, Containers (docker-stats-like) — each with "top apps" | **Adopt** (M9a) |
| 30 days of history at 1 h, 12 h, 24 h, 7 d, 30 d; top apps ranked hour by hour for 48 h | Same ranges everywhere; per-app hourly aggregates; "who used the most, hour by hour" | **Adopt** (M9a) |
| Projects tab: dev servers grouped by project folder, ports, memory, idle detection, "stopping them frees X and ports Y" | Dev servers and long-running processes grouped by project and worktree (cwd), listening ports, memory, idle time; the freeable figure is shown, the stop command is offered to copy (Fleetglass stays read-only) | **Adopt** (M9b) |
| Alerts for misbehaving apps: CPU busy for 10 min, memory growing for 1 h, disk or network hammering | Same rules as "What stands out" insights now; notifications arrive with alerts (M4) | **Adopt as insights** (M9b) |
| "When the Mac runs hot, Vitals names the apps behind it" | Heat attribution: when package temperature or power is high, the apps responsible at that moment | **Adopt** (M9b) |
| Roadmap: storage map, disk activity per drive, watchdog for runaway dev processes, agents tab | Storage map of configured roots (largest directories, growth), per-drive activity, runaway-process rule; agents tab already exists (M2) | **Adopt** (M9b) |
| MCP server (1.4): Claude, Codex, Cursor ask what is running, which app is behind a number, what is on a port; off until turned on | Read-only MCP endpoint, off by default, token-protected, same answers; never kills anything | **Adopt** (M9c) |
| Share card: dashboard exported as a 1200 x 630 image, light or dark | Share card PNG of a host or the fleet, private figures hidden on request | **Adopt** (M9c) |
| Settings: units (°C/°F, bytes/bits, per-core or whole CPU), tabs and sections shown, language, reset | Same, per browser; English and French first, i18n-ready for more | **Adopt** (M9c) |
| Glossary explaining each figure (load average, cached memory, swap, % CPU above 100…) | Inline "what this means" explanations on figures; Linux terms (PSI, iowait, steal, page cache, zram) | **Adopt** (M9c) |
| Honest totals ("unmeasured processes") | A visible "other / not attributed" share so per-app figures add up to the total | **Adopt** (M9a) |
| Quit / Force Quit, stop containers, fan control, per-app volume, Bluetooth, battery, browser tab split, menu bar, desktop widgets | Not for a read-only server monitor (actions) or not applicable (laptop hardware, desktop UI) | **No** (read-only principle; can be revisited later) |
| Pricing, cash back, license | Out of scope: no commercialisation for now | **No** |
| Privacy: local only, no analytics, update check only | Already Fleetglass's rule | Already done |

## Positioning check
Vitals answers "which app is using my Mac". Fleetglass answers "which agent, service or project is using my
server, and is everything healthy" — the same "name behind the figure" promise, extended to fleets, services,
uptime, security and AI cost. Vitals' own roadmap now lists "an Agents tab for Claude Code, Codex": the
agent-to-machine correlation remains Fleetglass's lead and must stay the headline.
