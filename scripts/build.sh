#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

mkdir -p dist
echo "==> tests"
go test ./...
echo "==> vet"
go vet ./...
echo "==> build"
go build -v -trimpath -o dist/r2go ./cmd/r2go
echo "Built: $ROOT/dist/r2go"
