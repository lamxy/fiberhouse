#!/usr/bin/env bash
#
# generate_swagger.sh — regenerate example_application/docs from swaggo
# annotations. NOT executed as part of any build/test/CI step in this repo;
# run it manually, on purpose, when you want to refresh the generated
# OpenAPI spec.
#
# Prerequisite (not installed by this script):
#   go install github.com/swaggo/swag/cmd/swag@latest
#
# See example_application/docs/README.md for the full rationale behind the
# flags below (why -g points at example_main/main.go, why only the Fiber
# handlers are annotated, and what doc.go looks like before/after this
# runs).

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if ! command -v swag >/dev/null 2>&1; then
  echo "error: swag CLI not found on PATH." >&2
  echo "install it with: go install github.com/swaggo/swag/cmd/swag@latest" >&2
  exit 1
fi

swag init \
  -g example_main/main.go \
  -d . \
  -o example_application/docs \
  --parseDependency \
  --parseInternal \
  --parseDepth 2

echo "Generated example_application/docs from swaggo annotations."
