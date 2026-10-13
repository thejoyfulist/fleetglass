# Exposing the cockpit safely

The installer leaves the cockpit at `127.0.0.1:7878` and never changes a firewall, public listener, Tailscale Serve or Caddy. Access it through an SSH tunnel, or place an operator-managed authenticated TLS reverse proxy in front of it. For a non-loopback `listen`, set a separate `read_token_file`; the server refuses to start without one. Keep ingest and read tokens distinct. Protect TLS, the reverse proxy and any Tailscale policy independently. Do not expose the OTLP receiver, helper socket or MCP listener publicly.

Set `allowed_hosts` to the exact hostname the browser sends to the proxy, with an optional port (for example `"allowed_hosts": ["cockpit.example.test:8444"]`). The default permits only `localhost` and literal loopback IPs. The proxy must preserve the browser-facing Host header.

A separate `status_listen` can expose only `/status` and the minimal public status API. Mark individual probes `public: true` only after reviewing their display names. The public response excludes host names, internal targets and detailed errors. Prometheus scraping follows the same read access rules as the private API; see [operations](OPERATIONS.md).
