# Claude Code and Codex setup

Fleetglass receives OTLP/HTTP on `127.0.0.1:4318`. Configure each CLI user separately; the receiver accepts metrics and logs, not gRPC or traces. Keep the listener local. OTLP is the preferred path because it provides exact token and timing events when the CLI emits them.

When no OTLP token is configured, the receiver accepts only HTTP Host headers naming `localhost` or a literal loopback IP. Use the loopback endpoint in local exporters.

For Claude Code, merge this `env` block into the user-level `~/.claude/settings.json`:

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:4318"
  }
}
```

For Codex, add to the user-level `~/.codex/config.toml`:

```toml
[otel]
log_user_prompt = false
exporter = { "otlp-http" = { endpoint = "http://127.0.0.1:4318/v1/logs", protocol = "binary" } }
metrics_exporter = { "otlp-http" = { endpoint = "http://127.0.0.1:4318/v1/metrics", protocol = "binary" } }
```

For fallback, configure `ai_log_roots` and grant only those roots read/traverse ACLs; see [installer flags](INSTALL.md) and [`deploy/AI-ACCESS.md`](../deploy/AI-ACCESS.md). This path reads JSONL incrementally and does not follow symlinks. Root and other-user CLI sessions need their own OTLP settings or explicitly approved log ACLs. Set `project_roots` and `worktree_roots` to improve project and worktree attribution. Codex quotas are CLI-reported. Claude quotas are reported by the status line when available, with configured-budget estimates as fallback. Cost is labelled by method; it is never presented as an exact bill. The read-only MCP connection is documented separately in [MCP.md](MCP.md) and remains off by default.

## Claude plan limits

The optional `fleetglass-statusline` binary reads Claude Code's documented [status-line JSON](https://code.claude.com/docs/en/statusline#available-data) from stdin. It extracts only the 5-hour, 7-day and optional spend-limit percentages and reset times, a hash of the session ID, and the Claude Code version. It sends OTLP/HTTP JSON to the local receiver. Install the release bundle first, then merge this into your user-level Claude Code `settings.json` yourself:

```json
{
  "statusLine": {
    "type": "command",
    "command": "fleetglass-statusline"
  }
}
```

If another command already owns the status line, keep its output and exit status by chaining it after `--`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "fleetglass-statusline -- my-existing-status-command"
  }
}
```

The wrapped command receives the same stdin bytes and its stdout is passed through unchanged. Without a wrapped command, Fleetglass prints a short percentage summary. `--endpoint http://127.0.0.1:PORT` or `FLEETGLASS_STATUSLINE_ENDPOINT` changes the local OTLP/HTTP endpoint; only loopback addresses are accepted. Delivery has a 250 ms timeout and errors are ignored so an unavailable receiver does not hold up the status line. The installer never edits user settings.

Claude Code documents the 5-hour and 7-day fields for **claude.ai Pro and Max subscribers only**. The optional spend-limit fields apply behind a Claude apps gateway with spend limits and require Claude Code v2.1.251 or later; the percentage may exceed 100%. All fields appear only after the first API response in a session, and each window can be absent. The status line runs locally without consuming tokens and updates on events (debounced by 300 ms), while Claude Code is open. An optional `refreshInterval` in `statusLine` can request periodic updates. See the [rate-limit fields and update behavior](https://code.claude.com/docs/en/statusline#spend-limit-fields). Fleetglass deliberately does not call the undocumented Claude OAuth usage endpoint or read Claude credentials: that route requires user OAuth credentials, impersonates the Claude Code client, is aggressively rate limited, and conflicts with the least-privilege approach and Anthropic's restrictions on third-party handling of Claude.ai credentials.

## Quota forecast

The Costs area has a text-only Forecast view. For each quota window, Fleetglass selects observations from the **last quarter of that window** with the same reset timestamp and source method. At least two recent observations are required; otherwise the pace and forecast are unknown. The base pace is the positive percentage-point change from the first to last selected observation divided by elapsed hours. With three or more observations, the base pace and the most recent interval pace receive equal weight. Decreases are treated as zero pace. This smoothing responds to acceleration without extrapolating a single jump alone.

Fleetglass projects from the last observation at that pace until reset. It shows a runout time and the hours locked until reset if projected use reaches 100% first; otherwise it shows approximate use at reset. The pace ratio compares the projected percentage points per hour with an even 100% spread over the full window. The last seven daily rows sum positive changes between observations on the same day and compare them with `100 / window days` percentage points per day; days over that allowance use the warning colour. Missing observations leave the day incomplete; daily bars are observations, not billing records. Every window with enough data gets a timeline and one-line verdict. The block timeline divides a seven-day window into 28 six-hour cells with day labels (short windows into 20 cells); its legend identifies projected use, the runout point, the shaded locked period, and reset. All values are forecasts, not provider guarantees.

## Where each number comes from

Session rows show a compact `src` tag: `otel` for OpenTelemetry, `log` for local session logs, `otel+log` when both were seen, and `proc` for a process-only session. Open the `src` cell to see the session detail: each metric has its own feed tag. For example, a mixed-source session can show `tokens 920 K otel`, `state working log`, `cpu 7.2% proc`, `speed ~15.2 tok/s est. (otel timing)`, and `~amount $7.92 est.`. A missing value is `-` without a feed tag. An OTLP session can retain a newer turn state from local logs; OTLP token observations take precedence over log totals for the same session. CPU and RSS are attributed from the matching process tree.

`~` marks estimates. Speed from request duration telemetry is measured; speed inferred from log timing is estimated. The `~amount` heading marks the API-equivalent amount as an estimate computed from token counts and the configured model price table, never an invoice. Unknown measurements remain unknown.

## File edit activity

Fleetglass records timestamps and names of observed tool calls, never tool arguments or file content. Claude Code file tools are `Edit`, `MultiEdit`, `Write`, and `NotebookEdit`. Codex file tools recognized are `apply_patch`, `functions.apply_patch`, `mcp__codex__apply_patch`, `edit_file`, `write_file`, and their `functions.` forms; shell commands are not classified from their arguments. Codex OTLP reports names on [`codex.tool_result` events](https://github.com/openai/codex/issues/17909). The same allowlist in `internal/ai/filetools.go` applies to OTLP tool events and Claude/Codex JSONL fallback.

For active sessions with tool data, a file change within `agent_no_edit_minutes` is **editing**. At or beyond the threshold, a positive token rate and token usage within two minutes yield **working, no file changes for Xm (possibly looping)** and a cyan WARN. Tool calls alone do not trigger the warning. Quiet sessions are **idle**; sessions without tool data are **unknown**. When no file change has been observed, the interval starts at the later of the session start and the first tool event Fleetglass observed. Older sessions then say **no file changes since tracking began Xm ago**. The default threshold is 20 minutes.

The configured-worktree scan runs every five minutes and counts a changed Git HEAD, dirty-file count, or index modification time as a repository file change, tagged `repo`. It compares successive cached scans; the first scan establishes a baseline. This catches some shell edits without inspecting command arguments. Changes reverted between scans, or changes that leave all three signals unchanged, may be missed. Without an attributed worktree, file changes through shell commands are not visible. A warning remains a review hint, not proof of a loop; the cockpit does not stop an agent or deliver alerts.

The API exposes `last_file_edit_at`, `last_tool_at`, `first_tool_at`, `minutes_since_file_edit`, `activity`, and `flag` on session list and detail responses. File-change provenance is `otel`, `log`, or `repo`.
