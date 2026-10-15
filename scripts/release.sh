#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
out=${1:-"$(pwd)/release"}
mkdir -p "$out"
out=$(cd "$out" && pwd)
description=$(git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --long 2>/dev/null || true)
if [ -n "$description" ]; then
  tag=${description%-*-*}
  count=${description#"$tag"-}; count=${count%%-*}
  version=${tag#v}
  if [ "$count" -gt 0 ]; then version="$version-dev.$count"; fi
else version=0.0.0-dev; fi
commit=$(git rev-parse --short=12 HEAD)
epoch=${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD)}
date=$(date -u -d "@$epoch" +%Y-%m-%dT%H:%M:%SZ)
ldflags="-s -w -X github.com/thejoyfulist/fleetglass/internal/version.Version=$version -X github.com/thejoyfulist/fleetglass/internal/version.Commit=$commit -X github.com/thejoyfulist/fleetglass/internal/version.Date=$date"
for arch in amd64 arm64; do
  name="fleetglass-v$version-linux-$arch"
  stage="$out/$name"
  mkdir -p "$stage"
  for bin in agent helper server statusline; do
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/fleetglass-$bin" "./cmd/fleetglass-$bin"
  done
  cp -R deploy docs assets "$stage/"
  cp LICENSE "$stage/"
  cp README.md "$stage/"
  (cd "$stage" && sha256sum fleetglass-agent fleetglass-helper fleetglass-server fleetglass-statusline > SHA256SUMS)
  tar --sort=name --owner=0 --group=0 --numeric-owner --mtime="@$epoch" -C "$out" -czf "$out/$name.tar.gz" "$name"
  rm -r "$stage"
done
(cd "$out" && sha256sum fleetglass-v*-linux-*.tar.gz > SHA256SUMS)
printf 'Built Fleetglass %s for linux/amd64 and linux/arm64 in %s\n' "$version" "$out"
