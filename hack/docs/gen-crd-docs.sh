#!/usr/bin/env bash
set -euo pipefail

mkdir -p docs

GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" go install github.com/elastic/crd-ref-docs@v0.3.0
GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" make generate >/dev/null

"$(go env GOPATH)/bin/crd-ref-docs" \
  --config hack/docs/crd-ref-docs.yaml \
  --source-path api \
  --renderer markdown \
  --output-mode single \
  --output-path docs/crd.md
