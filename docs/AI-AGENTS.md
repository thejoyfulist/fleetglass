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

Session rows show a compact `src` tag: `otel` for OpenTelemetry, `log` for local session logs, `otel+log` when both were seen, and `proc` for a process-only session. The session detail lists the feed chosen for state/turn, tokens, speed, model, project/worktree, and CPU/RSS. An OTLP session can retain a newer turn state from local logs; OTLP token observations take precedence over log totals for the same session. CPU and RSS are attributed from the matching process tree.

`~` marks estimates. Speed from request duration telemetry is measured; speed inferred from log timing is estimated. The `~amount` heading marks the API-equivalent amount as an estimate computed from token counts and the configured model price table, never an invoice. Unknown measurements remain unknown.
