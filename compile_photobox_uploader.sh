#!/usr/bin/env bash
set -euo pipefail

readonly ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly OUTPUT_DIR="${ROOT_DIR}/dist"
readonly OUTPUT_FILE="${OUTPUT_DIR}/photoupld-linux-amd64"

mkdir -p "${OUTPUT_DIR}"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -o "${OUTPUT_FILE}" "${ROOT_DIR}/cmd/photoupld"

echo "Built ${OUTPUT_FILE}"
