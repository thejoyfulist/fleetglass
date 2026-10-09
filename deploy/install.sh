#!/bin/sh
# Install an extracted Fleetglass release bundle. Run as root on a systemd Linux host.
set -eu
usage() { echo 'Usage: install.sh [--dry-run] [--upgrade] [--uninstall] [--journal-access] [--grant-ai-acls --ai-log-root kind:/absolute/path ...]' >&2; exit 2; }
dry=0 upgrade=0 uninstall=0 journal=0 acl=0 ai_roots=''
while [ "$#" -gt 0 ]; do
  case $1 in
    --dry-run) dry=1;; --upgrade) upgrade=1;; --uninstall) uninstall=1;;
    --journal-access) journal=1;; --grant-ai-acls) acl=1;;
    --ai-log-root) shift; [ "$#" -gt 0 ] || usage; ai_roots="$ai_roots $1";;
    *) usage;;
  esac
  shift
done
[ "$upgrade" -eq 0 ] || [ "$uninstall" -eq 0 ] || usage
[ "$acl" -eq 0 ] || [ -n "$ai_roots" ] || usage
for item in $ai_roots; do
  case $item in claude:/*|codex:/*) source_path=${item#*:};; *) usage;; esac
  case $source_path in *'/../'*|*'/..'|*'//'*) usage;; esac
done
root=${FLEETGLASS_ROOT:-/}
case $root in /*) ;; *) echo 'FLEETGLASS_ROOT must be absolute' >&2; exit 2;; esac
root=${root%/}; [ -n "$root" ] || root=/
path() { if [ "$root" = / ]; then printf '%s' "$1"; else printf '%s%s' "$root" "$1"; fi; }
say() { printf '%s\n' "$*"; }
run() { if [ "$dry" -eq 1 ]; then say "DRY RUN: $*"; else "$@"; fi; }
[ "$(uname -s)" = Linux ] || { echo 'Linux required' >&2; exit 1; }
case $(uname -m) in x86_64|aarch64) ;; *) echo 'Unsupported architecture' >&2; exit 1;; esac
command -v systemctl >/dev/null 2>&1 || { echo 'systemd/systemctl required' >&2; exit 1; }
[ -d "$(path /run/systemd/system)" ] || { echo 'systemd is not running' >&2; exit 1; }
if [ "$root" = / ] && [ "$dry" -eq 0 ] && [ "$(id -u)" -ne 0 ]; then echo 'Run as root' >&2; exit 1; fi
if [ "$uninstall" -eq 1 ]; then
  for unit in fleetglass-agent fleetglass-server fleetglass-helper; do run systemctl disable --now "$unit.service"; done
  for unit in fleetglass-agent fleetglass-server fleetglass-helper; do run rm -f "$(path "/etc/systemd/system/$unit.service")"; done
  run rm -f "$(path /etc/systemd/system/fleetglass-agent.service.d/state.conf)" "$(path /etc/systemd/system/fleetglass-server.service.d/state.conf)"
  for bin in agent server helper; do run rm -f "$(path "/usr/local/bin/fleetglass-$bin")"; done
  run systemctl daemon-reload
  if [ "$dry" -eq 1 ]; then say 'DRY RUN: Fleetglass units and binaries would be removed; configuration, tokens, data, backups, account and ACLs would be preserved.'
  else say 'Fleetglass units and binaries removed. Configuration, tokens, data, backups, account and ACLs are preserved for explicit review.'; fi
  exit 0
fi
self=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
bundle=$(CDPATH= cd -- "$self/.." && pwd)
[ -f "$bundle/SHA256SUMS" ] || { echo 'Bundle SHA256SUMS missing' >&2; exit 1; }
for bin in agent server helper; do [ -f "$bundle/fleetglass-$bin" ] || { echo "Missing fleetglass-$bin" >&2; exit 1; }; done
if [ "$dry" -eq 1 ]; then say 'DRY RUN: verify SHA-256 checksums for all three binaries'; else (cd "$bundle" && sha256sum -c SHA256SUMS); fi
say "Detected Linux $(uname -m) with systemd"
if [ "$dry" -eq 1 ]; then
  say 'DRY RUN: create fleetglass system group and unprivileged user if absent'
else
  getent group fleetglass >/dev/null || groupadd --system fleetglass
  getent passwd fleetglass >/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin --gid fleetglass fleetglass
fi
run install -d -m 0755 "$(path /etc/fleetglass)" "$(path /etc/systemd/system)" "$(path /usr/local/bin)"
run install -d -o fleetglass -g fleetglass -m 0750 "$(path /var/lib/fleetglass)" "$(path /var/lib/fleetglass/buffer)"
for bin in agent server helper; do run install -m 0755 "$bundle/fleetglass-$bin" "$(path "/usr/local/bin/fleetglass-$bin")"; done
for unit in agent server helper; do run install -m 0644 "$self/fleetglass-$unit.service" "$(path "/etc/systemd/system/fleetglass-$unit.service")"; done
for unit in agent server; do
  drop="$(path "/etc/systemd/system/fleetglass-$unit.service.d")"
  run install -d -m 0755 "$drop"
  if [ "$dry" -eq 1 ]; then say "DRY RUN: write $drop/state.conf with StateDirectory=fleetglass"; else printf '[Service]\nStateDirectory=fleetglass\nStateDirectoryMode=0750\n' > "$drop/state.conf"; fi
done
token=$(path /etc/fleetglass/token)
if [ ! -f "$token" ]; then
  if [ "$dry" -eq 1 ]; then say 'DRY RUN: generate private ingest token at /etc/fleetglass/token'; else (umask 077; od -An -N32 -tx1 /dev/urandom | tr -d ' \n' > "$token"); fi
else say 'Preserve existing ingest token'; fi
config=$(path /etc/fleetglass/config.json)
if [ ! -f "$config" ]; then
  say 'Detect Docker sockets, NVMe/SATA devices and common or failed systemd units'
  if [ "$dry" -eq 1 ]; then say 'DRY RUN: write detected loopback-only config at /etc/fleetglass/config.json'; else
    command -v python3 >/dev/null 2>&1 || { echo 'Python 3 required for initial configuration' >&2; exit 1; }
    units=$(systemctl list-unit-files --type=service --no-legend --no-pager 2>/dev/null | awk '{print $1}')
    failed=$(systemctl --failed --type=service --no-legend --no-pager 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i ~ /^[A-Za-z0-9_.@:-]+\.service$/) {print $i; break}}')
    FG_DETECT_ROOT="$root" FG_DETECT_UNITS="$units" FG_DETECT_FAILED="$failed" FG_AI_ROOTS="$ai_roots" python3 - "$config" <<'PY'
import json, os, re, socket, sys
root=os.environ['FG_DETECT_ROOT']
def exists(path): return os.path.exists(os.path.join(root,path.lstrip('/')))
units=set(os.environ.get('FG_DETECT_UNITS','').split())
failed=set(os.environ.get('FG_DETECT_FAILED','').split())
watch={'ssh.service','sshd.service','docker.service','containerd.service','cron.service','crond.service','tailscaled.service','caddy.service'}
watch=sorted((watch & units) | failed)
docker=[p for p in ('/var/run/docker.sock','/run/docker.sock') if exists(p)]
for base, dirs, files in os.walk(os.path.join(root,'run/user')) if exists('/run/user') else []:
    if base.count(os.sep) > os.path.join(root,'run/user').count(os.sep)+1: dirs[:]=[]
    if 'docker.sock' in files: docker.append('/'+os.path.relpath(os.path.join(base,'docker.sock'),root))
import glob
smart=[]
for pattern in ('/dev/nvme[0-9]*n[0-9]*','/dev/sd[a-z]'):
    for p in glob.glob(os.path.join(root,pattern.lstrip('/'))):
        name=os.path.basename(p)
        if re.fullmatch(r'(?:nvme[0-9]+n[0-9]+|sd[a-z]+)',name) and exists('/sys/block/'+name): smart.append('/dev/'+name)
roots=[]
for item in os.environ.get('FG_AI_ROOTS','').split():
    kind,sep,path=item.partition(':')
    if sep and kind in ('claude','codex') and path.startswith('/') and '..' not in path.split('/') and exists(path): roots.append({'kind':kind,'path':path})
name=socket.gethostname().split('.')[0]
name=''.join(c if (c.isascii() and c.isalnum()) or c in '-_' else '-' for c in name)[:64] or 'fleetglass-host'
c={'host_id':name,'server_url':'http://127.0.0.1:7878','listen':'127.0.0.1:7878','otlp_listen':'127.0.0.1:4318','token_file':'/etc/fleetglass/token','db_path':'/var/lib/fleetglass/fleetglass.db','buffer_dir':'/var/lib/fleetglass/buffer','helper_socket':'/run/fleetglass/helper.sock','systemd_units':watch,'docker_sockets':sorted(set(docker)),'smart_devices':sorted(set(smart)),'ai_log_roots':roots,'mcp_listen':'','status_listen':'','project_roots':[],'worktree_roots':[],'storage_roots':['/','/home','/var']}
with open(sys.argv[1],'x',encoding='utf-8') as f: json.dump(c,f,indent=2); f.write('\n')
PY
    chmod 0640 "$config"
    chown root:fleetglass "$config"
  fi
else say 'Preserve existing configuration and data'; fi
if [ "$journal" -eq 1 ]; then run usermod -a -G systemd-journal fleetglass; fi
if [ "$acl" -eq 1 ]; then
  command -v setfacl >/dev/null 2>&1 || { echo 'setfacl required for --grant-ai-acls' >&2; exit 1; }
  for item in $ai_roots; do
    case $item in claude:/*|codex:/*) source_path=${item#*:};; *) echo 'Invalid AI log root' >&2; exit 2;; esac
    [ -d "$(path "$source_path")" ] || { echo 'AI log root absent' >&2; exit 1; }
    parent=$(dirname "$source_path")
    while [ "$parent" != / ]; do run setfacl -m u:fleetglass:x "$(path "$parent")"; parent=$(dirname "$parent"); done
    run setfacl -R -m u:fleetglass:rX "$(path "$source_path")"
    if [ "$dry" -eq 1 ]; then say "DRY RUN: apply default read/traverse ACL to directories under $source_path"; else find "$(path "$source_path")" -type d -exec setfacl -m d:u:fleetglass:rX {} +; fi
  done
fi
run systemctl daemon-reload
for unit in fleetglass-helper fleetglass-server fleetglass-agent; do
  run systemctl enable "$unit.service"
  if [ "$upgrade" -eq 1 ]; then run systemctl restart "$unit.service"; else run systemctl start "$unit.service"; fi
done
say 'Local cockpit: http://127.0.0.1:7878 (loopback only)'
