#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
server_pid=
cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
mkdir -p "$tmp/root/run/systemd/system" "$tmp/mocks" "$tmp/www/releases/download/v1.2.3" "$tmp/bundle/fleetglass-v1.2.3-linux-amd64/deploy"
cat > "$tmp/mocks/uname" <<'MOCK'
#!/bin/sh
case $1 in -s) echo Linux;; -m) echo "${TEST_ARCH:-x86_64}";; esac
MOCK
cat > "$tmp/mocks/id" <<'MOCK'
#!/bin/sh
[ "$1" = -u ] && echo 0
MOCK
cat > "$tmp/mocks/systemctl" <<'MOCK'
#!/bin/sh
exit 0
MOCK
cat > "$tmp/bundle/fleetglass-v1.2.3-linux-amd64/deploy/install.sh" <<'MOCK'
#!/bin/sh
printf '%s\n' "$*" > "$TEST_INSTALL_ARGS"
MOCK
chmod +x "$tmp/mocks/"*
tar -czf "$tmp/www/releases/download/v1.2.3/fleetglass-v1.2.3-linux-amd64.tar.gz" -C "$tmp/bundle" fleetglass-v1.2.3-linux-amd64
(cd "$tmp/www/releases/download/v1.2.3" && sha256sum fleetglass-v1.2.3-linux-amd64.tar.gz > SHA256SUMS)
printf '{"tag_name":"v1.2.3"}\n' > "$tmp/www/releases/latest"
cat > "$tmp/serve.py" <<'PY'
import http.server, os, socketserver, sys
os.chdir(sys.argv[1])
class Server(socketserver.TCPServer): allow_reuse_address = True
with Server(('127.0.0.1', 0), http.server.SimpleHTTPRequestHandler) as server:
    with open(sys.argv[2], 'w') as output: output.write(str(server.server_address[1]))
    server.serve_forever()
PY
python3 "$tmp/serve.py" "$tmp/www" "$tmp/port" > "$tmp/server.log" 2>&1 &
server_pid=$!
attempts=0
while [ ! -f "$tmp/port" ] && [ "$attempts" -lt 50 ]; do sleep 0.1; attempts=$((attempts + 1)); done
[ -f "$tmp/port" ] || { echo 'Fixture HTTP server did not start' >&2; exit 1; }
export FLEETGLASS_BASE_URL="http://127.0.0.1:$(cat "$tmp/port")" FLEETGLASS_ROOT="$tmp/root" TEST_INSTALL_ARGS="$tmp/args" PATH="$tmp/mocks:$PATH"
sh install.sh --dry-run --upgrade
[ "$(cat "$tmp/args")" = '--dry-run --upgrade' ]
printf 'broken\n' >> "$tmp/www/releases/download/v1.2.3/fleetglass-v1.2.3-linux-amd64.tar.gz"
if sh install.sh --dry-run > "$tmp/bad.log" 2>&1; then echo 'Checksum mismatch accepted' >&2; exit 1; fi
grep -q 'checksum mismatch' "$tmp/bad.log"
if TEST_ARCH=ppc64 sh install.sh --dry-run > "$tmp/arch.log" 2>&1; then echo 'Unsupported architecture accepted' >&2; exit 1; fi
grep -q 'Unsupported architecture' "$tmp/arch.log"
rm "$tmp/www/releases/latest"
if sh install.sh --dry-run > "$tmp/release.log" 2>&1; then echo 'Missing release accepted' >&2; exit 1; fi
grep -q 'No published release found' "$tmp/release.log"
printf 'Bootstrap installer checks passed\n'
