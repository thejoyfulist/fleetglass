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
