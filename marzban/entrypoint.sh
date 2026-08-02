#!/bin/bash
set -e

XRAY_DIR="/usr/local/bin/xray-core"
mkdir -p "$XRAY_DIR"

if [ ! -f "$XRAY_DIR/xray" ]; then
    echo "Downloading latest Xray-core..."
    curl -sL "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip" -o /tmp/xray.zip
    unzip -o /tmp/xray.zip -d "$XRAY_DIR"
    chmod +x "$XRAY_DIR/xray"
    rm -f /tmp/xray.zip
    echo "Xray-core installed successfully"
else
    echo "Xray-core already present at $XRAY_DIR/xray"
fi

exec "$@"
