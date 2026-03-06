#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

protoc \
  --go_out="$ROOT/backend/internal/proto" \
  --go_opt=paths=source_relative \
  --go-grpc_out="$ROOT/backend/internal/proto" \
  --go-grpc_opt=paths=source_relative \
  -I "$ROOT/proto" \
  "$ROOT/proto/jock.proto"

echo "Proto generation complete."
