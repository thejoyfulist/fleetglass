# Read-only MCP endpoint

Fleetglass's MCP endpoint is **off by default**. It uses a separate Streamable HTTP listener, bound to a loopback IP, and requires its own bearer token. It exposes four read-only tools: `what_is_running`, `figure_attribution`, `what_is_on_port`, and `host_status`. There are no action tools. The result reflects observations, not a complete process or port inventory; configured project roots determine which processes have port attribution.

## Enable locally

Create a private token file (at least 32 characters, preferably 32 random bytes encoded as text), readable only by the Fleetglass server account. Add these fields to the **server** JSON configuration:

```json
{
  "mcp_listen": "127.0.0.1:7879",
  "mcp_token_file": "/absolute/private/path/fleetglass-mcp.token"
}
```

The endpoint is then `http://127.0.0.1:7879/mcp`. The listener rejects non-loopback bind addresses. If a client runs elsewhere, use an authenticated tunnel or a TLS reverse proxy controlled by the operator; never expose the loopback endpoint directly over an unencrypted network. The MCP token is separate from ingest and dashboard read tokens. Do not place it in this repository, a URL, shell history, or a committed config file.

Optional dashboard defaults can be set in the server config. Browser preferences take precedence and can be reset to these defaults:

```json
"display_defaults": {"language":"en", "temperature":"C", "network":"bytes", "cpu":"core"}
```

`language` accepts `en` or `fr`; `temperature` accepts `C` or `F`; `network` accepts `bytes` or `bits`; `cpu` accepts `core` or `machine`.

## Claude Code

In a private Claude Code `.mcp.json` file (project scope), use environment expansion for the header:

```json
{
  "mcpServers": {
    "fleetglass": {
      "type": "http",
      "url": "http://127.0.0.1:7879/mcp",
      "headers": {"Authorization": "Bearer ${FLEETGLASS_MCP_TOKEN}"}
    }
  }
}
```

Set `FLEETGLASS_MCP_TOKEN` in the Claude Code process environment from the private token file before starting Claude Code. The same connection can be added with `claude mcp add --transport http` and an Authorization header, but the JSON form keeps the token out of the CLI arguments. Claude Code's `/mcp` command shows connection status. See the [Claude Code MCP guide](https://code.claude.com/docs/en/mcp).

## Codex CLI or IDE

Add this section to a private `~/.codex/config.toml` or project-scoped `.codex/config.toml` (do not commit a project config containing credentials):

```toml
[mcp_servers.fleetglass]
url = "http://127.0.0.1:7879/mcp"
bearer_token_env_var = "FLEETGLASS_MCP_TOKEN"
```

Set `FLEETGLASS_MCP_TOKEN` in the Codex process environment from the private token file before starting Codex. `codex mcp list` shows the server. See the [Codex MCP configuration guide](https://learn.chatgpt.com/docs/extend/mcp?surface=cli).

## Protocol check

Use a local JSON-RPC client to POST `initialize`, then `tools/list`, then `tools/call` with `Content-Type: application/json`, `Accept: application/json, text/event-stream`, and `Authorization: Bearer <token>`. The `initialize` result advertises protocol version `2025-06-18`. Every advertised tool has `readOnlyHint: true`; unknown tool names return an error. The automated protocol test in `internal/api/mcp_test.go` exercises these calls, missing and incorrect tokens, an attempted action, and an untrusted Origin.
