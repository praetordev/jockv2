#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/../backend"

echo "Building jockd for $(go env GOOS)/$(go env GOARCH)..."
mkdir -p bin
go build -o bin/jockd ./cmd/jockd/
echo "Done: backend/bin/jockd"

echo "Building jockq..."
go build -o bin/jockq ./cmd/jockq/
echo "Done: backend/bin/jockq"

echo "Building jockmcp..."
go build -o bin/jockmcp ./cmd/jockmcp/
echo "Done: backend/bin/jockmcp"
