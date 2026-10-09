# Configuration reference

All fields are optional unless noted. Fleetglass reads one JSON file for the server, agent and helper on a local host. The example is [`deploy/config.example.json`](../deploy/config.example.json). Defaults below apply when a field is absent; an explicit empty array disables that list. Keep the file root-owned and group-readable by `fleetglass` only. Never put tokens in JSON when a token file can be used.

| Key | Default | Purpose / example |
|---|---|---|
| `host_id` | OS hostname | Stable host identity, e.g. `build-node`. |
| `labels` | `{}` | Fleet labels, e.g. `{"role":"build"}`. |
| `server_url` | `http://127.0.0.1:7878` | Agent ingest URL; remote URLs require HTTPS. |
| `listen` | `127.0.0.1:7878` | Private cockpit/API listener. Non-loopback requires a read token. |
| `token` | empty | Legacy ingest token; prefer `token_file`. |
| `token_file` | empty | Private ingest token path, e.g. `/etc/fleetglass/token`. |
| `ca_file` | empty | Absolute custom CA path for HTTPS agent connection. |
| `buffer_dir` | `/var/lib/fleetglass/buffer` | Agent's bounded offline batch buffer. |
| `read_token` | empty | Cockpit/API bearer token; prefer `read_token_file`. |
| `read_token_file` | empty | Private read-token path. |
| `mcp_listen` | empty/off | Loopback-only read-only MCP listener, e.g. `127.0.0.1:7879`. |
| `mcp_token_file` | empty | Private token path, required with `mcp_listen`. |
| `live_max_subscribers` | `32` | Maximum concurrent SSE subscribers. |
| `proc_root`, `sys_root` | `/proc`, `/sys` | Probe roots; alternate roots are for fixtures. |
| `systemd_units` | `[]` | Watched units, e.g. `["ssh.service"]`. |
| `docker_sockets` | `["/var/run/docker.sock"]` | Rootful and rootless socket paths. |
| `tls_domains` | `[]` | Domains for certificate expiry checks. |
| `smart_devices` | `[]` | Allowlisted `/dev/nvme*` or `/dev/sd*` devices for the helper. |
| `helper_socket` | `/run/fleetglass/helper.sock` | Group-restricted helper Unix socket. |
| `db_path` | `fleetglass.db` | SQLite path, absolute in installed config. |
| `otlp_listen` | `127.0.0.1:4318` | OTLP/HTTP receiver; non-loopback requires `otlp_token`. |
| `otlp_token` | empty | Optional OTLP bearer token. |
| `ai_log_roots` | `[]` | Optional `{kind: "claude" or "codex", path: "/absolute/root"}` entries. |
| `agent_commands` | `["claude","codex"]` | Process names eligible for agent attribution. |
| `worktree_roots` | `[]` | Roots laid out as project/worktree. |
| `project_roots` | `[]` | Roots of local projects, one project per child directory. |
| `storage_roots` | `["/","/home","/var"]` | Bounded depth-two storage map roots. |
| `backup_sets` | `[]` | Monitored backup files; schema below. |
| `status_files` | `[]` | Monitored JSON status files; schema below. |
| `log_markers` | `[]` | Bounded tail checks for configured text; schema below. |
| `repository_roots` | `[]` | Local Git repository parent directories. |
| `local_only_repositories` | `[]` | Names intentionally without an upstream. |
| `excluded_repositories` | `[]` | Repository names omitted from scans. |
| `internal_probes` | `[]` | TCP/HTTP checks; schema below. |
| `http_probes` | `[]` | External HTTP(S) checks; schema below. |
| `status_listen` | empty/off | Separate minimal public status listener. |
| `apt_history_path` | `/var/log/apt` | Read-only APT history directory. |
| `energy_price_per_kwh` | `0` | CPU package cost estimate price. |
| `energy_currency` | `EUR` | ISO three-letter currency code. |
| `ai_prices` | `{}` | Per-model USD prices overriding built-in dated prices. Each entry has `input`, `output`, `cache_write`, `cache_read` per million tokens. |
| `ai_subscriptions` | `[]` | `{kind: "claude" or "codex", monthly: 150, currency: "EUR"}` entries. Legacy `monthly_usd` remains accepted as USD. Do not combine both forms in one entry. |
| `display_currency` | `USD` | Default cockpit cost currency. Each browser can choose USD or EUR in Display settings. |
| `exchange_rates` | absent | Optional static rates: `{"date":"2026-10-08","rates":{"USD":1.16,"GBP":0.87}}`. Rates are currency units per EUR; EUR is always 1. No network access is needed. |
| `exchange_rate_source` | empty/off | `ecb` enables one daily request to the public ECB XML feed, with a four-second timeout and SQLite cache. The latest cached rates remain available if a request fails. |
| `claude_budget_5h`, `claude_budget_week` | `0` | Optional estimated Claude token budgets. |
| `display_defaults` | English, Celsius, bytes, per-core CPU | `{language: "en" or "fr", temperature: "C" or "F", network: "bytes" or "bits", cpu: "core" or "machine"}`. |
| `backup_dir` | empty/off | Absolute directory for one online database backup per UTC day. |
| `backup_retention_days` | `0` | Positive number required when `backup_dir` is set. Only Fleetglass-named daily files are pruned. |
| `update_check_url` | empty/off | HTTPS latest-release JSON endpoint (`tag_name`); no host identifiers sent. Keep off while the repository is private. |

Nested monitoring entries:

- `backup_sets`: `name`, absolute `pattern` (directory or glob), `max_age_seconds` required; optional `warn_age_seconds`, `min_size_bytes`, `suffixes` (e.g. `[".db"]`).
- `status_files`: `name`, absolute `path`, dot-separated `field_path`, `max_age_seconds` required; optional `ok_values`, `warn_values`, `crit_values` string arrays and `missing_state` (`warn`, `crit`, `unknown`).
- `log_markers`: `name`, `path`, `contains`, `max_age_seconds` required; optional `display_name`, `present_state`, `absent_state`, `change_event`.
- `internal_probes`: `name`, `kind` (`tcp`/`http`), `target`, `timeout_ms` required; optional `display_name`, `public`.
- `http_probes`: `name`, `display_name`, `url` required; optional `method` (`GET`/`HEAD`, default GET), `expected_status` (default any 2xx), `body_keyword`, `timeout_ms` (default 5000), `interval_seconds` (default 60, minimum 30), `follow_redirects`, `public`.

The helper and agent must receive identical configured source lists when split into separate JSON files. Private paths are never auto-discovered except the installer’s Docker and SMART hardware detection. Refer to [system access](../deploy/SYSTEM-ACCESS.md) for monitored files and helper fallback behaviour.

Costs in the REST API retain `amount` and `currency` (or their legacy USD fields) and include a `money` object with `original`, `currency`, `converted`, `display_currency`, and, when converted, `rate`, `rate_source`, and `rate_date`. The subscriptions, energy, cost series, and session detail endpoints use the same money fields. Clients can request `?display_currency=EUR`; a missing rate returns `converted: null`, so the cockpit displays the original currency and an unavailable-rate note. Built-in API model prices and CLI-reported costs are USD. `/metrics` exports token and machine metrics, not monetary metrics, so its semantics are unchanged. The current MCP tools expose no monetary figure; browser currency preferences do not affect them.
