#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
export ENG_DEMO_ROOT="$PWD/demo/.root"

go build -o bin/eng ./cmd/eng
[ -d demo/node_modules ] || npm install --prefix demo --no-audit --no-fund

go run ./scripts/demoroot -out "$ENG_DEMO_ROOT"
vhs demo/terminal.tape

# the web run answers cards, so it gets a fresh root
go run ./scripts/demoroot -out "$ENG_DEMO_ROOT"
node demo/capture.mjs
