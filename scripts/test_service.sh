#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[test] formatting check (gofmt)"
fmt_out="$(gofmt -l .)"
if [[ -n "$fmt_out" ]]; then
  echo "[test] gofmt found unformatted files:"
  echo "$fmt_out"
  echo "[test] run 'gofmt -w' on the files above and re-run this script."
  exit 1
fi

echo "[test] go vet"
go vet ./...

echo "[test] go test"
go test -race -count=1 ./...

echo "[test] completed successfully"

