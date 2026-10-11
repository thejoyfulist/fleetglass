#!/bin/sh
# Fetch and verify a Fleetglass release before invoking its bundled installer.
set -eu

fail() { printf 'Fleetglass installer: %s\n' "$*" >&2; exit 1; }
[ "$(uname -s)" = Linux ] || fail 'Linux is required.'
case $(uname -m) in
  x86_64) arch=amd64 ;;
  aarch64) arch=arm64 ;;
  *) fail 'Unsupported architecture (use Linux amd64 or arm64).' ;;
esac
command -v systemctl >/dev/null 2>&1 || fail 'systemd/systemctl is required.'
[ -d "${FLEETGLASS_ROOT:-}/run/systemd/system" ] || fail 'systemd must be running.'
[ "$(id -u)" -eq 0 ] || fail 'Run as root (for example, pipe to sudo sh).'
for program in curl sha256sum tar mktemp; do
  command -v "$program" >/dev/null 2>&1 || fail "$program is required."
done

base=${FLEETGLASS_BASE_URL:-https://github.com/thejoyfulist/fleetglass}
base=${base%/}
if [ -n "${FLEETGLASS_VERSION:-}" ]; then
  version=$FLEETGLASS_VERSION
else
  if [ -n "${FLEETGLASS_BASE_URL:-}" ]; then
    api=$base/releases/latest
  else
    api=https://api.github.com/repos/thejoyfulist/fleetglass/releases/latest
  fi
  response=$(curl -fsSL "$api") || fail 'No published release found. Set FLEETGLASS_VERSION for a specific release.'
  version=$(printf '%s\n' "$response" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\(v[0-9][0-9.]*\)".*/\1/p' | head -n 1)
  [ -n "$version" ] || fail 'Latest release response has no valid tag_name.'
fi
case $version in
  v[0-9]*) case $version in *[!v0-9.]*) fail 'Invalid release version.';; esac ;;
  *) fail 'Invalid release version (expected vMAJOR.MINOR.PATCH).' ;;
esac
case ${version#v} in *.*.*) ;; *) fail 'Invalid release version (expected vMAJOR.MINOR.PATCH).' ;; esac

tmp=$(mktemp -d) || fail 'Cannot create temporary directory.'
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
name=fleetglass-$version-linux-$arch
url=$base/releases/download/$version
curl -fsSL "$url/$name.tar.gz" -o "$tmp/$name.tar.gz" || fail 'Release archive download failed.'
curl -fsSL "$url/SHA256SUMS" -o "$tmp/SHA256SUMS" || fail 'SHA256SUMS download failed.'
(cd "$tmp" && sha256sum -c SHA256SUMS --ignore-missing | grep -Fx "$name.tar.gz: OK" >/dev/null) || fail 'Archive checksum mismatch or missing SHA256SUMS entry.'
tar -xzf "$tmp/$name.tar.gz" -C "$tmp" || fail 'Archive extraction failed.'
[ -f "$tmp/$name/deploy/install.sh" ] || fail 'Release bundle has no deploy/install.sh.'
sh "$tmp/$name/deploy/install.sh" "$@"
