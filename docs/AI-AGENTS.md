# Claude Code and Codex setup

Fleetglass receives OTLP/HTTP on `127.0.0.1:4318`. Configure each CLI user separately; the receiver accepts metrics and logs, not gRPC or traces. Keep the listener local. OTLP is the preferred path because it provides exact token and timing events when the CLI emits them.

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

For fallback, configure `ai_log_roots` and grant only those roots read/traverse ACLs; see [installer flags](INSTALL.md) and [`deploy/AI-ACCESS.md`](../deploy/AI-ACCESS.md). This path reads JSONL incrementally and does not follow symlinks. Root and other-user CLI sessions need their own OTLP settings or explicitly approved log ACLs. Set `project_roots` and `worktree_roots` to improve project and worktree attribution. Quotas are CLI-reported for Codex and estimated from configured budgets for Claude. Cost is labelled by method; it is never presented as an exact bill. The read-only MCP connection is documented separately in [MCP.md](MCP.md) and remains off by default.

## Where each number comes from

Session rows show a compact `src` tag: `otel` for OpenTelemetry, `log` for local session logs, `otel+log` when both were seen, and `proc` for a process-only session. Open the `src` cell to see the session detail: each metric has its own feed tag. For example, a mixed-source session can show `tokens 920 K otel`, `state working log`, `cpu 7.2% proc`, `speed ~15.2 tok/s est. (otel timing)`, and `~amount $7.92 est.`. A missing value is `-` without a feed tag. An OTLP session can retain a newer turn state from local logs; OTLP token observations take precedence over log totals for the same session. CPU and RSS are attributed from the matching process tree.

`~` marks estimates. Speed from request duration telemetry is measured; speed inferred from log timing is estimated. The `~amount` heading marks the API-equivalent amount as an estimate computed from token counts and the configured model price table, never an invoice. Unknown measurements remain unknown.

## File edit activity

Fleetglass records timestamps and names of observed tool calls, never tool arguments or file content. Claude Code file tools are `Edit`, `MultiEdit`, `Write`, and `NotebookEdit`. Codex file tools recognized are `apply_patch`, `functions.apply_patch`, `mcp__codex__apply_patch`, `edit_file`, `write_file`, and their `functions.` forms; shell commands are not classified from their arguments. Codex OTLP reports names on [`codex.tool_result` events](https://github.com/openai/codex/issues/17909). The same allowlist in `internal/ai/filetools.go` applies to OTLP tool events and Claude/Codex JSONL fallback.

For active sessions with tool data, a file change within `agent_no_edit_minutes` is **editing**. At or beyond the threshold, a positive token rate and token usage within two minutes yield **working, no file changes for Xm (possibly looping)** and a cyan WARN. Tool calls alone do not trigger the warning. Quiet sessions are **idle**; sessions without tool data are **unknown**. When no file change has been observed, the interval starts at the later of the session start and the first tool event Fleetglass observed. Older sessions then say **no file changes since tracking began Xm ago**. The default threshold is 20 minutes.

The configured-worktree scan runs every five minutes and counts a changed Git HEAD, dirty-file count, or index modification time as a repository file change, tagged `repo`. It compares successive cached scans; the first scan establishes a baseline. This catches some shell edits without inspecting command arguments. Changes reverted between scans, or changes that leave all three signals unchanged, may be missed. Without an attributed worktree, file changes through shell commands are not visible. A warning remains a review hint, not proof of a loop; the cockpit does not stop an agent or deliver alerts.

The API exposes `last_file_edit_at`, `last_tool_at`, `first_tool_at`, `minutes_since_file_edit`, `activity`, and `flag` on session list and detail responses. File-change provenance is `otel`, `log`, or `repo`.
