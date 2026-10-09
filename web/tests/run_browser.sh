#!/usr/bin/env bash
set -euo pipefail
if [[ -n "${FLEETGLASS_HEAVY_LOCK:-}" && "${FLEETGLASS_LOCK_HELD:-}" != 1 ]]; then
  export FLEETGLASS_LOCK_HELD=1
  exec flock "$FLEETGLASS_HEAVY_LOCK" "$0" "$@"
fi
repo_dir="$(cd "$(dirname "$0")/../.." && pwd)"
run_dir="$(mktemp -d)"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]]; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  rm -rf "$run_dir"
}
trap cleanup EXIT
port="$(python3 - <<'PY'
import socket
s=socket.socket();s.bind(('127.0.0.1',0));print(s.getsockname()[1]);s.close()
PY
)"
base="http://127.0.0.1:$port"
cat > "$run_dir/config.json" <<JSON
{"host_id":"test-host","listen":"127.0.0.1:$port","server_url":"$base","token":"fixture-ingest","proc_root":"$run_dir/proc","sys_root":"$run_dir/sys","docker_sockets":[],"helper_socket":"$run_dir/helper.sock","db_path":"$run_dir/fleetglass.db"}
JSON
mkdir -p "$run_dir/proc" "$run_dir/sys"
cd "$repo_dir"
go build -o "$run_dir/server" ./cmd/fleetglass-server
go build -o "$run_dir/agent" ./cmd/fleetglass-agent
"$run_dir/server" -config "$run_dir/config.json" > "$run_dir/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 50); do
  if curl -fsS "$base/api/v1/auth" >/dev/null 2>&1; then break; fi
  sleep .1
done
curl -fsS "$base/api/v1/auth" >/dev/null
"$run_dir/agent" -once -config "$run_dir/config.json" > "$run_dir/agent.log" 2>&1
python3 web/tests/seed.py "$base" fixture-ingest
shots_dir="${1:-$(mktemp -d -t fleetglass-browser-shots-XXXXXX)}"
python3 web/tests/browser.py "$base" "$shots_dir"
