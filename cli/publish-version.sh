#!/usr/bin/env bash
set -euo pipefail

# Publish one isolated xparse-cli version directory.
# This script intentionally never updates the shared installers or latest.
# Usage: ./publish-version.sh <version> [test|prod]
# Example: ./publish-version.sh v2.1.0 prod

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "Usage: $0 <version> [test|prod]"
  echo "Example: $0 v2.1.0 prod"
  exit 1
fi

VERSION="$1"
RELEASE_KIND="${2:-test}"
if [[ ! "$VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "Invalid version: use letters, numbers, dots, underscores, or hyphens."
  exit 1
fi
if [ "$VERSION" = "latest" ]; then
  echo "Refusing to publish the reserved latest directory."
  exit 1
fi
if [ "$RELEASE_KIND" != "test" ] && [ "$RELEASE_KIND" != "prod" ]; then
  echo "Invalid release kind: use test or prod."
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
PINNED_INSTALL_SH="${DIST_DIR}/.install.${VERSION}.$$.sh"
PINNED_INSTALL_PS1="${DIST_DIR}/.install.${VERSION}.$$.ps1"

if [ "$RELEASE_KIND" = "prod" ]; then
  CONNECTOR_CONFIG="${CONNECTOR_DIR}/cli.json"
else
  CONNECTOR_CONFIG="${CONNECTOR_DIR}/cli.test.json"
fi

cleanup() {
  rm -f "${SKILL_ARCHIVE_TMP}" "${PINNED_INSTALL_SH}" "${PINNED_INSTALL_PS1}"
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
  "${CONNECTOR_CONFIG}"
  "${CONNECTOR_DIR}/connector-meta.json"
  "${CONNECTOR_DIR}/icon.png"
  "${CONNECTOR_DIR}/marketplace-entry.json"
  "${SKILLS_DIR}/xparse-parse/SKILL.md"
)
if [ "$RELEASE_KIND" = "test" ]; then
  REQUIRED_FILES+=(
    "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.sh"
    "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.sh"
    "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.ps1"
    "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.ps1"
  )
fi
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

sed \
  -e "s/default: \"latest\"/default: \"${VERSION}\"/" \
  -e "s/XPARSER_VERSION:-latest/XPARSER_VERSION:-${VERSION}/" \
  "${SCRIPT_DIR}/install/install.sh" > "${PINNED_INSTALL_SH}"
sed \
  -e "s/default: \"latest\"/default: \"${VERSION}\"/" \
  -e "s/else { \"latest\" }/else { \"${VERSION}\" }/" \
  "${SCRIPT_DIR}/install/install.ps1" > "${PINNED_INSTALL_PS1}"

for file in "${REQUIRED_FILES[@]}"; do
  if [ ! -f "$file" ]; then
    echo "Missing release artifact: $file"
    echo "Run ./build.sh ${VERSION} first."
    exit 1
  fi
done
if [ "$RELEASE_KIND" = "prod" ]; then
  grep -Fq "/${VERSION}/install.sh" "${CONNECTOR_CONFIG}" ||
    {
      echo "Production Connector config is not pinned to ${VERSION}."
      exit 1
    }
  grep -Fq '"authUrlDomain": "api.textin.com"' "${CONNECTOR_CONFIG}" ||
    {
      echo "Production Connector config does not use api.textin.com."
      exit 1
    }
  grep -Fq '"XPARSE_OAUTH_CLIENT_ID": "cli_textin_xparse_workbuddy"' \
    "${CONNECTOR_CONFIG}" ||
    {
      echo "Production Connector config does not use the WorkBuddy OAuth client."
      exit 1
    }
  if grep -Fq '/latest/' "${CONNECTOR_CONFIG}"; then
    echo "Production Connector config must not reference latest."
    exit 1
  fi
fi

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

upload "${PINNED_INSTALL_SH}" "${BASE_PATH}/install.sh" \
  "text/plain; charset=utf-8"
upload "${PINNED_INSTALL_PS1}" "${BASE_PATH}/install.ps1" \
  "text/plain; charset=utf-8"
upload "${CONNECTOR_CONFIG}" "${BASE_PATH}/workbuddy-cli.json" \
  "application/json; charset=utf-8"
upload "${CONNECTOR_DIR}/connector-meta.json" \
  "${BASE_PATH}/workbuddy-connector-meta.json" "application/json; charset=utf-8"
upload "${CONNECTOR_DIR}/icon.png" "${BASE_PATH}/workbuddy-icon.png"
upload "${CONNECTOR_DIR}/marketplace-entry.json" \
  "${BASE_PATH}/workbuddy-marketplace-entry.json" "application/json; charset=utf-8"
upload "${SKILL_ARCHIVE}" "${BASE_PATH}/workbuddy-xparse-parse.zip"
if [ "$RELEASE_KIND" = "test" ]; then
  upload "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.sh" \
    "${BASE_PATH}/enable-workbuddy-test.sh" "text/plain; charset=utf-8"
  upload "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.sh" \
    "${BASE_PATH}/restore-workbuddy-production.sh" "text/plain; charset=utf-8"
  upload "${CONNECTOR_TEST_DIR}/enable-workbuddy-test.ps1" \
    "${BASE_PATH}/enable-workbuddy-test.ps1" "text/plain; charset=utf-8"
  upload "${CONNECTOR_TEST_DIR}/restore-workbuddy-production.ps1" \
    "${BASE_PATH}/restore-workbuddy-production.ps1" "text/plain; charset=utf-8"
fi
for file in "${BINARY_FILES[@]}"; do
  upload "${DIST_DIR}/${file}" "${BASE_PATH}/${file}"
done

echo "Published isolated version: ${VERSION}"
echo "Release kind: ${RELEASE_KIND}"
echo "No shared installer or rolling release directory was updated."
