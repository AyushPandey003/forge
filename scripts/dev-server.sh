#!/usr/bin/env bash
# Run the Go API natively against the dockerised infra (docker-compose.dev.yml).
set -euo pipefail
cd "$(dirname "$0")/.."

export DATABASE_URL="postgres://slashdocs:slashdocs@localhost:5433/slashdocs?sslmode=disable"
export SLASHDOCS_MIGRATE=true
export SLASHDOCS_KV_EVENTS_MODE=notify
export BLOB_BUCKET=slashdocs-documents
export BLOB_ENDPOINT=http://localhost:9000
export BLOB_FORCE_PATH_STYLE=true
export BLOB_ACCESS_KEY_ID=blobadmin
export BLOB_SECRET_ACCESS_KEY=blobadmin
export BLOB_REGION=us-east-1
export SLASHDOCS_APP_ORIGIN=http://localhost:8080
export SLASHDOCS_RENDER_ORIGIN=http://127.0.0.1:8080
export SLASHDOCS_OIDC_ISSUER_URL=http://localhost:5556/dex
export SLASHDOCS_OIDC_CLIENT_ID=slashdocs
export SLASHDOCS_OIDC_CLIENT_SECRET=slashdocs-dev-secret
export SLASHDOCS_OIDC_PROVIDER_NAME=Dex
export SESSION_SECRET=dev-only-session-secret-0123456789abcdef
# Forge design engine: Google Stitch + Katalyst DESIGN.md (see docs/forge.md)
# Auth: STITCH_API_KEY, or gcloud (personal account) with a quota project.
export STITCH_API_KEY="${STITCH_API_KEY:-}"
export STITCH_USE_GCLOUD="${STITCH_USE_GCLOUD:-true}"
export STITCH_PROJECT="${STITCH_PROJECT:-forge-stitch-261104}"
export FORGE_DESIGN_MD="${FORGE_DESIGN_MD:-design/katalyst.DESIGN.md}"
export FORGE_STITCH_PROJECT_ID="${FORGE_STITCH_PROJECT_ID:-3828330232627373588}"
export FORGE_STITCH_DESIGN_SYSTEM="${FORGE_STITCH_DESIGN_SYSTEM:-assets/dab0e246b3314cee95056f31ce8da142}"

H=$(node frontend/scripts/bridge-source-hash.mjs)
go build -ldflags "-X github.com/razorpay/slashdocs/api/internal/server.BuildBridgeSourceHash=$H" -o bin/server.exe ./api/cmd/slashdocs-server
exec ./bin/server.exe
