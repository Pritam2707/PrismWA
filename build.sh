#!/usr/bin/env bash
set -euo pipefail

CGO_ENABLED=0 go build -ldflags="-s -w -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore" -o prismbot .
chmod +x prismbot
echo "Built: ./prismbot"

if [[ "${1:-}" == "termux" || "${1:-}" == "arm64" ]]; then
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore" -o prismbot-termux-arm64 .
    chmod +x prismbot-termux-arm64
    echo "Built: ./prismbot-termux-arm64"
fi
