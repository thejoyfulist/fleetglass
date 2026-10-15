#!/bin/sh
set -eu
archive=${1:?Pass a locally built linux/amd64 release archive}
case $archive in /*) ;; *) archive=$(pwd)/$archive;; esac
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/bundle" "$tmp/root/run/systemd/system" "$tmp/root/dev" "$tmp/root/sys/block/nvme0n1" "$tmp/root/sys/block/sda" "$tmp/root/var/run" "$tmp/root/run/user/1000" "$tmp/mocks"
mkdir -p "$tmp/root/home/demo/.claude/projects"
tar -xzf "$archive" -C "$tmp/bundle"
bundle=$(find "$tmp/bundle" -mindepth 1 -maxdepth 1 -type d -print -quit)
: > "$tmp/root/dev/nvme0n1"
: > "$tmp/root/dev/sda"
: > "$tmp/root/var/run/docker.sock"
: > "$tmp/root/run/user/1000/docker.sock"
cat > "$tmp/mocks/systemctl" <<'MOCK'
#!/bin/sh
case "$1" in
  list-unit-files) printf 'ssh.service enabled\ndocker.service enabled\n';;
  --failed) printf 'example-failed.service failed\n';;
  *) printf '%s\n' "$*" >> "$FG_SYSTEMCTL_LOG";;
esac
MOCK
cat > "$tmp/mocks/getent" <<'MOCK'
#!/bin/sh
exit 0
MOCK
cat > "$tmp/mocks/install" <<'MOCK'
#!/bin/sh
args=''
while [ "$#" -gt 0 ]; do
  case $1 in -o|-g) shift 2;; *) args="$args '$1'"; shift;; esac
done
eval "exec /usr/bin/install $args"
MOCK
cat > "$tmp/mocks/chown" <<'MOCK'
#!/bin/sh
exit 0
MOCK
chmod +x "$tmp/mocks/systemctl" "$tmp/mocks/getent" "$tmp/mocks/install" "$tmp/mocks/chown"
export FG_SYSTEMCTL_LOG="$tmp/systemctl.log" FLEETGLASS_ROOT="$tmp/root" PATH="$tmp/mocks:$PATH"
sh "$bundle/deploy/install.sh" --dry-run > "$tmp/dry-fresh.txt"
grep -q 'verify SHA-256' "$tmp/dry-fresh.txt"
grep -q 'generate private ingest token' "$tmp/dry-fresh.txt"
sh "$bundle/deploy/install.sh" > "$tmp/install.txt"
python3 - "$tmp/root/etc/fleetglass/config.json" <<'PY'
import json,sys
c=json.load(open(sys.argv[1]))
assert c['listen']=='127.0.0.1:7878'
assert c['mcp_listen']==''
assert c['docker_sockets']==['/run/user/1000/docker.sock','/var/run/docker.sock']
assert c['smart_devices']==['/dev/nvme0n1','/dev/sda']
assert c['systemd_units']==['docker.service','example-failed.service','ssh.service']
PY
[ -f "$tmp/root/etc/systemd/system/fleetglass-server.service.d/state.conf" ]
[ -x "$tmp/root/usr/local/bin/fleetglass-statusline" ]
sh "$bundle/deploy/install.sh" --dry-run > "$tmp/dry-existing.txt"
grep -q 'Preserve existing ingest token' "$tmp/dry-existing.txt"
sh "$bundle/deploy/install.sh" --upgrade --dry-run > "$tmp/dry-upgrade.txt"
grep -q 'systemctl restart fleetglass-agent.service' "$tmp/dry-upgrade.txt"
sh "$bundle/deploy/install.sh" --upgrade > "$tmp/upgrade.txt"
grep -q 'restart fleetglass-server.service' "$tmp/systemctl.log"
sh "$bundle/deploy/install.sh" --uninstall --dry-run > "$tmp/dry-uninstall.txt"
grep -q 'systemctl disable --now' "$tmp/dry-uninstall.txt"
sh "$bundle/deploy/install.sh" --dry-run --journal-access --grant-ai-acls --ai-log-root claude:/home/demo/.claude/projects > "$tmp/dry-access.txt"
grep -q 'systemd-journal fleetglass' "$tmp/dry-access.txt"
grep -q 'setfacl -R' "$tmp/dry-access.txt"
cp "$bundle/fleetglass-agent" "$tmp/original-agent"
printf 'tamper' >> "$bundle/fleetglass-agent"
if sh "$bundle/deploy/install.sh" > "$tmp/tamper.txt" 2>&1; then echo 'Tampered binary accepted' >&2; exit 1; fi
grep -q 'FAILED' "$tmp/tamper.txt"
mv "$tmp/original-agent" "$bundle/fleetglass-agent"
sh "$bundle/deploy/install.sh" --uninstall > "$tmp/uninstall.txt"
[ ! -e "$tmp/root/usr/local/bin/fleetglass-server" ]
[ ! -e "$tmp/root/usr/local/bin/fleetglass-statusline" ]
[ -f "$tmp/root/etc/fleetglass/config.json" ]
[ -f "$tmp/root/etc/fleetglass/token" ]
printf 'Installer fixture checks passed\n'
cat "$tmp/dry-fresh.txt" "$tmp/dry-existing.txt" "$tmp/dry-upgrade.txt" "$tmp/dry-uninstall.txt" "$tmp/dry-access.txt" > "${FG_INSTALL_TRANSCRIPT:-/tmp/fleetglass-install-transcript.txt}"
