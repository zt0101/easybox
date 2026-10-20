#!/usr/bin/env bash
# Build frontend + backend inside the dev container and run the panel in the foreground.
set -euo pipefail
cd /src
if [ ! -d frontend/node_modules/.bin ]; then
  (cd frontend && npm ci)
fi
if [ "${SKIP_FE:-0}" != "1" ]; then
  (cd frontend && npm run build)
fi
go build -o /tmp/x-ui .
exec /tmp/x-ui
