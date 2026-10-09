# Milestone 9d operations

## Prometheus

The main server exposes `GET /metrics` using Prometheus text format. It follows the private read API's authentication: when `read_token` is set, send the same bearer token to `/metrics`. Keep the endpoint on a trusted interface or behind an authenticated reverse proxy. It exports host liveness, selected host measurements, probe states, current app CPU and memory, and AI token counts from the last hour. Labels contain host, app, probe, agent and metric names; raw command lines and secrets are excluded.

```yaml
scrape_configs:
  - job_name: fleetglass
    scrape_interval: 30s
    metrics_path: /metrics
    static_configs:
      - targets: ["127.0.0.1:7878"]
    authorization:
      type: Bearer
      credentials_file: /path/to/read-token
```

Omit `authorization` only if the private read API intentionally has no token. Use a file with appropriate permissions; never put the token in a repository. No helper endpoint or host privilege change is required.

## Session and speed data

`GET /api/v1/hosts/{host}/agents/{session}` returns the session card. `GET /api/v1/hosts/{host}/ai/speed?range=1h|24h` returns output speed and request timing by agent and model. Add `session={hashed session ID}` for a single session. `GET /api/v1/hosts/{host}/ai/subscriptions` compares this calendar month's API equivalent with configured `ai_subscriptions` per agent. A missing subscription is returned as `null`, never as zero. `GET /api/v1/hosts/{host}/worktrees` returns the latest bounded inventory.

OTLP durations and time to first token are shown only when the CLI emits an allowlisted numeric attribute. Output tokens per second is exact only when the CLI reports that rate directly, or when output tokens, request duration and time to first token arrive on the same event. A speed inferred from two session log message timestamps is labelled `estimated`. Missing measurements remain null. Raw speed observations are retained with the existing seven day AI usage retention; the 1 h and 24 h views use raw rows. Errors and retries are counted only from received OTLP events. A process is marked left behind only after Fleetglass witnessed its ancestry while the agent process was alive; a process discovered after reparenting cannot be attributed safely.

Worktree sizes use an I/O-niced `du` scan bounded to 2 s per tree. An incomplete scan is labelled incomplete and excluded from reclaimable totals. Merge status is unknown when the default remote branch cannot be resolved. The estimated age uses the linked worktree `.git` file modification time; a normal repository with a `.git` directory has unknown age. Fleetglass never removes a worktree or stops a process.

## Installable app

The web manifest and service worker cache the static shell and hashed assets, never API responses. Offline startup displays the time of the last successful host sample stored in this browser. Installing the app does not enable notifications. The status page does not register the worker.
