#!/bin/sh
set -e
if [ -d "/app/dist-fresh" ]; then
  cp -rn /app/dist-fresh/. /app/dist/
fi
exec "$@"
