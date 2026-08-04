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
SKILLS_DIR="${SCRIPT_DIR}/../skills"
BASE_PATH="xparse-cli/${VERSION}"
UPLOAD_SCRIPT="${SCRIPT_DIR}/upload.sh"
SKILL_ARCHIVE="${DIST_DIR}/workbuddy-xparse-parse.zip"
SKILL_ARCHIVE_TMP="${DIST_DIR}/.workbuddy-xparse-parse.$$.zip"

cleanup() {
  rm -f "${SKILL_ARCHIVE_TMP}"
}
trap cleanup EXIT

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
  "${SKILLS_DIR}/xparse-parse/SKILL.md"
)
for file in "${BINARY_FILES[@]}"; do
  REQUIRED_FILES+=("${DIST_DIR}/${file}")
done

if ! command -v zip >/dev/null 2>&1; then
  echo "Missing release dependency: zip"
  exit 1
fi
(
  cd "${SKILLS_DIR}"
  zip -qr "${SKILL_ARCHIVE_TMP}" xparse-parse
)
mv -f "${SKILL_ARCHIVE_TMP}" "${SKILL_ARCHIVE}"
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
  local content_type="${3:-}"
  if [ "${DRY_RUN:-0}" = "1" ]; then
    echo "[DRY RUN] ${source_file} -> ${destination} (${content_type:-default})"
    return
  fi
  /bin/bash "$UPLOAD_SCRIPT" "$source_file" "$destination" "$content_type"
}

upload "${SCRIPT_DIR}/install/install.sh" "${BASE_PATH}/install.sh" \
  "text/plain; charset=utf-8"
upload "${SCRIPT_DIR}/install/install.ps1" "${BASE_PATH}/install.ps1" \
  "text/plain; charset=utf-8"
upload "${CONNECTOR_DIR}/cli.test.json" "${BASE_PATH}/workbuddy-cli.json" \
  "application/json; charset=utf-8"
upload "${CONNECTOR_DIR}/connector-meta.json" \
  "${BASE_PATH}/workbuddy-connector-meta.json" "application/json; charset=utf-8"
upload "${CONNECTOR_DIR}/icon.png" "${BASE_PATH}/workbuddy-icon.png"
upload "${CONNECTOR_DIR}/marketplace-entry.json" \
  "${BASE_PATH}/workbuddy-marketplace-entry.json" "application/json; charset=utf-8"
upload "${SKILL_ARCHIVE}" "${BASE_PATH}/workbuddy-xparse-parse.zip"
upload "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.sh" \
  "${BASE_PATH}/enable-workbuddy-test.sh" "text/plain; charset=utf-8"
upload "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.sh" \
  "${BASE_PATH}/restore-workbuddy-production.sh" "text/plain; charset=utf-8"
upload "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.ps1" \
  "${BASE_PATH}/enable-workbuddy-test.ps1" "text/plain; charset=utf-8"
upload "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.ps1" \
  "${BASE_PATH}/restore-workbuddy-production.ps1" "text/plain; charset=utf-8"
for file in "${BINARY_FILES[@]}"; do
  upload "${DIST_DIR}/${file}" "${BASE_PATH}/${file}"
done

echo "Published isolated version: ${VERSION}"
echo "No shared installer or rolling release directory was updated."
