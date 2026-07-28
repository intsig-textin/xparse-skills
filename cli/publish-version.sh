#!/usr/bin/env bash
set -euo pipefail

# Publish one isolated xparse-cli version directory.
# This script intentionally never updates the shared installers or latest.
# Usage: ./publish-version.sh <version>
# Example: ./publish-version.sh workbuddy-test

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 workbuddy-test"
  exit 1
fi

VERSION="$1"
if [[ ! "$VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "Invalid version: use letters, numbers, dots, underscores, or hyphens."
  exit 1
fi
if [ "$VERSION" = "latest" ]; then
  echo "Refusing to publish the reserved latest directory."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="${SCRIPT_DIR}/dist"
CONNECTOR_DIR="${SCRIPT_DIR}/../connector"
CONNECTOR_TEST_DIR="${CONNECTOR_DIR}/test"
BASE_PATH="xparse-cli/${VERSION}"
UPLOAD_SCRIPT="${SCRIPT_DIR}/upload.sh"

BINARY_FILES=(
  "xparse-cli-darwin-amd64"
  "xparse-cli-darwin-arm64"
  "xparse-cli-linux-amd64"
  "xparse-cli-linux-arm64"
  "xparse-cli-windows-amd64.exe"
  "xparse-cli-windows-arm64.exe"
)

REQUIRED_FILES=(
  "${SCRIPT_DIR}/install/install.sh"
  "${SCRIPT_DIR}/install/install.ps1"
  "${CONNECTOR_DIR}/cli.test.json"
  "${CONNECTOR_DIR}/connector-meta.json"
  "${CONNECTOR_DIR}/icon.png"
  "${CONNECTOR_DIR}/marketplace-entry.json"
  "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.sh"
  "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.sh"
  "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.ps1"
  "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.ps1"
)
for file in "${BINARY_FILES[@]}"; do
  REQUIRED_FILES+=("${DIST_DIR}/${file}")
done
for file in "${REQUIRED_FILES[@]}"; do
  if [ ! -f "$file" ]; then
    echo "Missing release artifact: $file"
    echo "Run ./build.sh ${VERSION} first."
    exit 1
  fi
done

upload() {
  local source_file="$1"
  local destination="$2"
  if [ "${DRY_RUN:-0}" = "1" ]; then
    echo "[DRY RUN] ${source_file} -> ${destination}"
    return
  fi
  /bin/bash "$UPLOAD_SCRIPT" "$source_file" "$destination"
}

upload "${SCRIPT_DIR}/install/install.sh" "${BASE_PATH}/install.sh"
upload "${SCRIPT_DIR}/install/install.ps1" "${BASE_PATH}/install.ps1"
upload "${CONNECTOR_DIR}/cli.test.json" "${BASE_PATH}/workbuddy-cli.json"
upload "${CONNECTOR_DIR}/connector-meta.json" \
  "${BASE_PATH}/workbuddy-connector-meta.json"
upload "${CONNECTOR_DIR}/icon.png" "${BASE_PATH}/workbuddy-icon.png"
upload "${CONNECTOR_DIR}/marketplace-entry.json" \
  "${BASE_PATH}/workbuddy-marketplace-entry.json"
upload "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.sh" \
  "${BASE_PATH}/enable-workbuddy-test.sh"
upload "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.sh" \
  "${BASE_PATH}/restore-workbuddy-production.sh"
upload "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.ps1" \
  "${BASE_PATH}/enable-workbuddy-test.ps1"
upload "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.ps1" \
  "${BASE_PATH}/restore-workbuddy-production.ps1"
for file in "${BINARY_FILES[@]}"; do
  upload "${DIST_DIR}/${file}" "${BASE_PATH}/${file}"
done

echo "Published isolated version: ${VERSION}"
echo "No shared installer or rolling release directory was updated."
