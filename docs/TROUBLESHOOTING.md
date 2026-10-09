# Troubleshooting

- **No host appears:** inspect `systemctl status fleetglass-agent fleetglass-server` and their journal messages. Verify that both use the same ingest token file and that the agent's `server_url` is reachable. The agent buffer holds only about ten minutes.
- **A source is unknown:** Fleetglass reports unreadable or absent sources as unknown. Confirm the agent can traverse configured roots, read the system journal if explicitly granted membership, and reach the helper socket. Recheck `smart_devices` and Docker socket paths after hardware or daemon changes.
- **Agent telemetry absent:** verify the CLI user has OTLP enabled, uses HTTP/protobuf and sends to the local `/v1/metrics` and `/v1/logs` endpoints. For Codex use user-level config. No prompts or raw OTLP attributes should appear in the database.
- **Cockpit login fails:** use the read token, not the ingest or MCP token. Reverse proxies must preserve cookies and forward HTTPS. Browser local storage contains preferences, not tokens.
- **Config rejected after upgrade:** check JSON spelling against [configuration](CONFIGURATION.md). Unknown keys are rejected, while omitted new keys keep defaults. Startup logs name optional keys available to older configs.
- **Backup fails:** verify `backup_dir` exists or can be created by `fleetglass`, has free space and is not the destination of an existing snapshot. Run the manual backup command to an unused path.
- **Offline PWA:** the installable shell caches static assets, never API data. It shows the timestamp of the last successful sample known to that browser. Restore network access to resume live readings.
