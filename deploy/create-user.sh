#!/bin/sh
set -eu

# Run manually as root after reviewing the requested account and group.
groupadd --system fleetglass
useradd --system --no-create-home --shell /usr/sbin/nologin --gid fleetglass fleetglass
