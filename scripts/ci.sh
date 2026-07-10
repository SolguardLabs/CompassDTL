#!/usr/bin/env bash
set -euo pipefail

gofmt -l cmd src >/tmp/compassdtl_gofmt_check.txt
if [ -s /tmp/compassdtl_gofmt_check.txt ]; then
  cat /tmp/compassdtl_gofmt_check.txt
  exit 1
fi

go test ./...
npm run check
npm test
npm run loc
