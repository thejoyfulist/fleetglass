# Install and upgrade

Fleetglass supports Linux on amd64 and arm64 with systemd. The release bundle contains three static binaries, unit files, an example config, the installer and SHA-256 sums. Installation needs root, `systemctl`, `sha256sum`, `install`, `od`, Python 3 for first-time detection, and optionally `setfacl`. Review the script and bundle before running it.

```sh
# After obtaining a release from the project's release page:
sha256sum -c SHA256SUMS             # verifies the downloaded tarball
tar -xzf fleetglass-vVERSION-linux-ARCH.tar.gz
cd fleetglass-vVERSION-linux-ARCH
sudo sh deploy/install.sh
```

The installer verifies each bundled binary again. It creates the unprivileged `fleetglass` account, installs units and `StateDirectory` drop-ins, generates a private ingest token, detects Docker sockets, NVMe/SATA devices and common or failed systemd services, then starts helper, server and agent. Its only listeners are `127.0.0.1:7878` for the cockpit and `127.0.0.1:4318` for OTLP. Configuration and token live in `/etc/fleetglass`; database and agent buffer live under `/var/lib/fleetglass`. The helper is the only root service. No firewall or reverse proxy is changed.

Run `sudo sh deploy/install.sh --dry-run` to print the planned actions. Run `sudo sh deploy/install.sh --upgrade` from a newer verified bundle to replace binaries and restart services while retaining config and data. A regular rerun is idempotent and preserves config, token and data. Run `sudo sh deploy/install.sh --uninstall` to stop and remove units and binaries; it deliberately retains config, tokens, data, account and ACLs for manual review. Back up the database before upgrading; see [backup and restore](BACKUP.md).

`--journal-access` explicitly adds the agent account to `systemd-journal`. `--grant-ai-acls --ai-log-root claude:/home/USER/.claude/projects --ai-log-root codex:/home/USER/.codex/sessions` grants read-only ACLs and records those roots in a new config. Review the paths first. Existing configs are never rewritten, so add new AI roots manually before applying ACLs to an existing installation. Prefer OTLP over the log fallback. For remote hosts, follow [multi-server setup](MULTI-SERVER.md) instead of exposing the default loopback listener.

Older JSON configs remain valid. Server and agent log the names of new optional keys they support, without logging values. Add those keys selectively using the [configuration reference](CONFIGURATION.md). `--version` on any binary prints its semantic tag version, commit and UTC build date.
