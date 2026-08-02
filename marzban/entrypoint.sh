#!/bin/bash
set -e

XRAY_DIR="/usr/local/bin/xray-core"
mkdir -p "$XRAY_DIR"

if [ ! -f "$XRAY_DIR/xray" ]; then
    echo "Downloading latest Xray-core..."
    python3 - <<'PYEOF'
import urllib.request
import zipfile
import os
import sys

xray_dir = "/usr/local/bin/xray-core"
url = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip"
zip_path = "/tmp/xray.zip"

try:
    urllib.request.urlretrieve(url, zip_path)
except Exception as e:
    print(f"Failed to download Xray-core: {e}", file=sys.stderr)
    sys.exit(1)

with zipfile.ZipFile(zip_path, 'r') as z:
    for name in z.namelist():
        z.extract(name, xray_dir)

xray_bin = os.path.join(xray_dir, "xray")
if os.path.exists(xray_bin):
    os.chmod(xray_bin, 0o755)
    print("Xray-core installed successfully")
else:
    print("Xray binary not found in archive", file=sys.stderr)
    sys.exit(1)

os.remove(zip_path)
PYEOF
else
    echo "Xray-core already present at $XRAY_DIR/xray"
fi

exec "$@"
