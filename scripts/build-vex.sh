#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VEX_SRC="${VEX_SRC:-$SCRIPT_DIR/../../vex}"
DEST="$SCRIPT_DIR/../backend/bin"

cd "$VEX_SRC"

echo "Building vex for $(go env GOOS)/$(go env GOARCH)..."
mkdir -p "$DEST"
go build -o "$DEST/vex" ./cmd/vex/
echo "Done: backend/bin/vex"
