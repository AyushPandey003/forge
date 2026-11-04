#!/usr/bin/env bash
# Rebuild the frontend (optional) and restart the containerised API.
#   scripts/restart.sh          # frontend + api
#   scripts/restart.sh server   # api only
set -euo pipefail
cd "$(dirname "$0")/.."
COMPOSE=(docker compose -f docker-compose.yml -f docker-compose.dev.yml)

if [ "${1:-}" != "server" ]; then
  (cd frontend && npx vite build >/tmp/vite.log 2>&1 && npx vite build --mode cli >>/tmp/vite.log 2>&1 \
    && npm run -s build:bridge >>/tmp/vite.log 2>&1 && node scripts/sync-dist.mjs) || { tail -30 /tmp/vite.log; exit 1; }
fi
node frontend/scripts/bridge-source-hash.mjs > frontend/.bridge-source-hash

"${COMPOSE[@]}" up -d postgres blob dex >/dev/null 2>&1
"${COMPOSE[@]}" up -d --force-recreate api >/dev/null 2>&1
for i in $(seq 1 150); do
  curl -sf -m 2 localhost:8080/readyz >/dev/null && { echo READY; exit 0; }
  sleep 2
done
"${COMPOSE[@]}" logs --tail 30 api; exit 1
