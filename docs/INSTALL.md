# Install and upgrade

Fleetglass supports Linux amd64 and arm64 with systemd. Installation needs root, `systemctl`, `curl`, `sha256sum`, `tar`, `mktemp`, `install`, and `od`; Python 3 is needed for first-time host detection. `setfacl` is optional for local AI log access.

## One-line install

Review [the bootstrap script](../install.sh), then run:

```sh
curl -fsSL https://raw.githubusercontent.com/thejoyfulist/fleetglass/main/install.sh | sudo sh
```

Preview without installing:

```sh
curl -fsSL https://raw.githubusercontent.com/thejoyfulist/fleetglass/main/install.sh | sudo sh -s -- --dry-run
```

The bootstrap script checks Linux, systemd, root, and architecture; reads the latest release tag from GitHub (or uses `FLEETGLASS_VERSION=vX.Y.Z`); downloads the matching archive and `SHA256SUMS`; verifies the archive; extracts into a temporary directory; and runs its bundled `deploy/install.sh` with your arguments. It removes the temporary files on exit. Set `FLEETGLASS_VERSION` in the root environment to pin a release. If no release exists yet, build from source or use a reviewed release bundle.

For upgrades, add `--upgrade` to the one-line command. Back up the database first; see [backup and restore](BACKUP.md).

## Manual verified install

Download the Linux archive and top-level `SHA256SUMS` from the same [release](https://github.com/thejoyfulist/fleetglass/releases):

```sh
sha256sum -c SHA256SUMS
tar -xzf fleetglass-vVERSION-linux-ARCH.tar.gz
cd fleetglass-vVERSION-linux-ARCH
sudo sh deploy/install.sh --dry-run
sudo sh deploy/install.sh
```

The bundled installer verifies each binary again. It creates the unprivileged `fleetglass` account, installs units and `StateDirectory` drop-ins, generates a private ingest token, detects Docker sockets, NVMe/SATA devices and common or failed systemd services, then starts helper, server and agent. Its only listeners are `127.0.0.1:7878` for the cockpit and `127.0.0.1:4318` for OTLP. Configuration and token live in `/etc/fleetglass`; database and agent buffer live under `/var/lib/fleetglass`. The helper is the only root service. No firewall or reverse proxy is changed.

Run `sudo sh deploy/install.sh --upgrade` from a newer verified bundle to replace binaries and restart services while retaining config and data. A regular rerun preserves config, token and data. Run `sudo sh deploy/install.sh --uninstall` to stop and remove units and binaries; it deliberately retains config, tokens, data, account and ACLs for manual review.

`--journal-access` explicitly adds the agent account to `systemd-journal`. `--grant-ai-acls --ai-log-root claude:/home/USER/.claude/projects --ai-log-root codex:/home/USER/.codex/sessions` grants read-only ACLs and records those roots in a new config. Review the paths first. Existing configs are never rewritten, so add new AI roots manually before applying ACLs to an existing installation. Prefer OTLP over the log fallback. For remote hosts, follow [multi-server setup](MULTI-SERVER.md) instead of exposing the default loopback listener.

Older JSON configs remain valid. Server and agent log the names of new optional keys they support, without logging values. Add those keys selectively using the [configuration reference](CONFIGURATION.md). `--version` on any binary prints its semantic tag version, commit and UTC build date.
