#!/bin/sh
# Builds and runs the API inside the dev container (see docker-compose.dev.yml).
set -eu
cd /src
H=$(cat frontend/.bridge-source-hash 2>/dev/null || true)
if [ "${#H}" -ne 64 ]; then
  echo "missing frontend/.bridge-source-hash; run scripts/restart.sh on the host" >&2
  exit 1
fi
go build -buildvcs=false -ldflags "-X github.com/razorpay/slashdocs/api/internal/server.BuildBridgeSourceHash=$H" -o /tmp/server ./api/cmd/slashdocs-server
exec /tmp/server
