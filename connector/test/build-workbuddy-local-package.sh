#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPOSITORY_ROOT="$(CDPATH= cd -- "${SCRIPT_DIR}/../.." && pwd)"
OUTPUT_DIR="${REPOSITORY_ROOT}/cli/dist/local-packages"
PACKAGE_VERSION=""
NPM_VERSION="2.2.1-beta.2"
NPM_REGISTRY="https://registry.npmmirror.com"

usage() {
  cat <<'USAGE'
Usage: build-workbuddy-local-package.sh [options]

Options:
  --output-dir PATH    Destination directory for the ZIP.
  --version VERSION    Immutable Connector package version label.
USAGE
}

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-dir)
      [ "$#" -ge 2 ] || fail "--output-dir 缺少路径。"
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --version)
      [ "$#" -ge 2 ] || fail "--version 缺少版本。"
      PACKAGE_VERSION="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "未知参数：$1"
      ;;
  esac
done

if [ -z "${PACKAGE_VERSION}" ]; then
  PACKAGE_VERSION="v2.2.1-beta.2-local-$(date -u +%Y%m%d%H%M%S)"
fi
case "${PACKAGE_VERSION}" in
  *[!A-Za-z0-9._-]*|'') fail "版本只能包含字母、数字、点、下划线或连字符。" ;;
esac

command -v zip >/dev/null 2>&1 || fail "缺少 zip，无法生成本地包。"
if command -v shasum >/dev/null 2>&1; then
  SHA256_COMMAND="shasum -a 256"
elif command -v sha256sum >/dev/null 2>&1; then
  SHA256_COMMAND="sha256sum"
else
  fail "缺少 shasum 或 sha256sum。"
fi

SKILLS_COMMIT="$(git -C "${REPOSITORY_ROOT}" rev-parse HEAD)"
if [ -n "$(git -C "${REPOSITORY_ROOT}" status --porcelain)" ]; then
  SKILLS_DIRTY=true
else
  SKILLS_DIRTY=false
fi

mkdir -p "${OUTPUT_DIR}"
OUTPUT_DIR="$(CDPATH= cd -- "${OUTPUT_DIR}" && pwd)"
PACKAGE_NAME="workbuddy-xparse-connector-${PACKAGE_VERSION}"
PACKAGE_PATH="${OUTPUT_DIR}/${PACKAGE_NAME}.zip"
[ ! -e "${PACKAGE_PATH}" ] || fail "输出已存在：${PACKAGE_PATH}"

TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/workbuddy-xparse-package.XXXXXX")"
cleanup() {
  if [ -d "${TEMP_ROOT}" ]; then
    find "${TEMP_ROOT}" -depth -delete 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

PACKAGE_ROOT="${TEMP_ROOT}/${PACKAGE_NAME}"
ASSET_DIR="${PACKAGE_ROOT}/assets"
mkdir -p "${ASSET_DIR}" "${PACKAGE_ROOT}/scripts"

cp "${REPOSITORY_ROOT}/connector/cli.test.json" \
  "${ASSET_DIR}/workbuddy-cli.json"
cp "${REPOSITORY_ROOT}/connector/connector-meta.json" \
  "${ASSET_DIR}/workbuddy-connector-meta.json"
cp "${REPOSITORY_ROOT}/connector/icon.png" \
  "${ASSET_DIR}/workbuddy-icon.png"
cp "${REPOSITORY_ROOT}/connector/marketplace-entry.json" \
  "${ASSET_DIR}/workbuddy-marketplace-entry.json"
(
  cd "${REPOSITORY_ROOT}/skills"
  zip -qr "${ASSET_DIR}/workbuddy-xparse-parse.zip" xparse-parse
)
cp "${SCRIPT_DIR}/enable-workbuddy-test.sh" \
  "${PACKAGE_ROOT}/scripts/enable-workbuddy-test.sh"
cp "${SCRIPT_DIR}/enable-workbuddy-test.ps1" \
  "${PACKAGE_ROOT}/scripts/enable-workbuddy-test.ps1"
cp "${SCRIPT_DIR}/restore-workbuddy-production.sh" "${PACKAGE_ROOT}/restore.sh"
cp "${SCRIPT_DIR}/restore-workbuddy-production.ps1" "${PACKAGE_ROOT}/restore.ps1"
chmod 0755 "${PACKAGE_ROOT}/scripts/enable-workbuddy-test.sh" \
  "${PACKAGE_ROOT}/restore.sh"

cat > "${PACKAGE_ROOT}/install.sh" <<INSTALL
#!/bin/sh
set -eu

PACKAGE_ROOT="\$(CDPATH= cd -- "\$(dirname -- "\$0")" && pwd)"
if command -v shasum >/dev/null 2>&1; then
  (cd "\${PACKAGE_ROOT}" && LC_ALL=C shasum -a 256 -c SHA256SUMS)
elif command -v sha256sum >/dev/null 2>&1; then
  (cd "\${PACKAGE_ROOT}" && LC_ALL=C sha256sum -c SHA256SUMS)
else
  printf '错误：缺少 shasum 或 sha256sum，无法校验本地包。\n' >&2
  exit 1
fi
command -v node >/dev/null 2>&1 || {
  printf '错误：缺少 Node.js 18 或更高版本，WorkBuddy 状态未修改。\n' >&2
  exit 1
}
command -v npm >/dev/null 2>&1 || {
  printf '错误：缺少 npm，WorkBuddy 状态未修改。\n' >&2
  exit 1
}
NODE_MAJOR="\$(env -u NODE_OPTIONS node -p "process.versions.node.split('.')[0]")"
if [ "\${NODE_MAJOR}" -lt 18 ]; then
  printf '错误：Node.js 版本过低（%s），WorkBuddy 状态未修改。\n' "\${NODE_MAJOR}" >&2
  exit 1
fi
case "\$(uname -s):\$(uname -m)" in
  Darwin:x86_64) PLATFORM_PACKAGE="xparse-cli-darwin-amd64" ;;
  Darwin:arm64) PLATFORM_PACKAGE="xparse-cli-darwin-arm64" ;;
  Linux:x86_64) PLATFORM_PACKAGE="xparse-cli-linux-amd64" ;;
  Linux:aarch64|Linux:arm64) PLATFORM_PACKAGE="xparse-cli-linux-arm64" ;;
  *)
    printf '错误：当前平台不在 xparse-cli 包支持列表中，WorkBuddy 状态未修改。\n' >&2
    exit 1
    ;;
esac
for NPM_PACKAGE in xparse-cli "\${PLATFORM_PACKAGE}"; do
  RESOLVED_VERSION="\$(env -u NODE_OPTIONS npm view "\${NPM_PACKAGE}@${NPM_VERSION}" version \
    --registry="${NPM_REGISTRY}" 2>/dev/null || true)"
  if [ "\${RESOLVED_VERSION}" != "${NPM_VERSION}" ]; then
    printf '错误：%s@%s 尚未同步到 %s，WorkBuddy 状态未修改。\n' \
      "\${NPM_PACKAGE}" "${NPM_VERSION}" "${NPM_REGISTRY}" >&2
    exit 1
  fi
done
env -u NODE_OPTIONS \\
  XPARSE_TEST_ASSET_DIR="\${PACKAGE_ROOT}/assets" \\
  XPARSER_VERSION="${PACKAGE_VERSION}" \\
  XPARSE_NPM_VERSION="${NPM_VERSION}" \\
  XPARSE_NPM_REGISTRY="${NPM_REGISTRY}" \\
  sh "\${PACKAGE_ROOT}/scripts/enable-workbuddy-test.sh"
INSTALL
chmod 0755 "${PACKAGE_ROOT}/install.sh"

cat > "${PACKAGE_ROOT}/install.ps1" <<POWERSHELL
\$ErrorActionPreference = "Stop"
\$PackageRoot = Split-Path -Parent \$MyInvocation.MyCommand.Path
\$HadNodeOptions = Test-Path Env:NODE_OPTIONS
\$OriginalNodeOptions = \$env:NODE_OPTIONS
try {
    Remove-Item Env:NODE_OPTIONS -ErrorAction SilentlyContinue
\$ChecksumPath = Join-Path \$PackageRoot "SHA256SUMS"
\$ChecksumLines = [System.IO.File]::ReadAllLines(\$ChecksumPath)
foreach (\$Line in \$ChecksumLines) {
    if (\$Line -notmatch '^([0-9a-fA-F]{64})  (.+)\$') {
        throw "SHA256SUMS 包含无效行：\${Line}"
    }
    \$ExpectedHash = \$Matches[1].ToLowerInvariant()
    \$RelativePath = \$Matches[2] -replace '^\\./', ''
    \$FilePath = Join-Path \$PackageRoot (\$RelativePath -replace '/', '\\')
    \$ActualHash = (Get-FileHash -LiteralPath \$FilePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if (\$ActualHash -ne \$ExpectedHash) {
        throw "本地包校验失败：\${RelativePath}"
    }
    Write-Host "\${RelativePath}: OK"
}

if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    throw "缺少 Node.js 18 或更高版本，WorkBuddy 状态未修改。"
}
if (-not (Get-Command npm.cmd -ErrorAction SilentlyContinue)) {
    throw "缺少 npm，WorkBuddy 状态未修改。"
}
\$NodeMajor = [int](& node -p "process.versions.node.split('.')[0]")
if (\$NodeMajor -lt 18) {
    throw "Node.js 版本过低（\${NodeMajor}），WorkBuddy 状态未修改。"
}
\$PlatformPackage = switch (\$env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "xparse-cli-windows-amd64" }
    "ARM64" { "xparse-cli-windows-arm64" }
    default { throw "当前 Windows 架构不在 xparse-cli 包支持列表中，WorkBuddy 状态未修改。" }
}
foreach (\$NpmPackage in @("xparse-cli", \$PlatformPackage)) {
    \$ResolvedVersion = (& npm.cmd view "\${NpmPackage}@${NPM_VERSION}" version "--registry=${NPM_REGISTRY}" 2>\$null)
    if (\$LASTEXITCODE -ne 0 -or \$ResolvedVersion.Trim() -ne "${NPM_VERSION}") {
        throw "\${NpmPackage}@${NPM_VERSION} 尚未同步到 ${NPM_REGISTRY}，WorkBuddy 状态未修改。"
    }
}

\$env:XPARSE_TEST_ASSET_DIR = Join-Path \$PackageRoot "assets"
\$env:XPARSER_VERSION = "${PACKAGE_VERSION}"
\$env:XPARSE_NPM_VERSION = "${NPM_VERSION}"
\$env:XPARSE_NPM_REGISTRY = "${NPM_REGISTRY}"
& (Join-Path \$PackageRoot "scripts\\enable-workbuddy-test.ps1")
} finally {
    if (\$HadNodeOptions) {
        \$env:NODE_OPTIONS = \$OriginalNodeOptions
    } else {
        Remove-Item Env:NODE_OPTIONS -ErrorAction SilentlyContinue
    }
}
POWERSHELL

add_utf8_bom() {
  target_path="$1"
  header="$(od -An -tx1 -N3 "${target_path}" | tr -d '[:space:]')"
  if [ "${header}" = "efbbbf" ]; then
    return
  fi
  bom_path="${target_path}.utf8-bom.$$"
  {
    printf '\357\273\277'
    cat "${target_path}"
  } > "${bom_path}"
  mv "${bom_path}" "${target_path}"
}

find "${PACKAGE_ROOT}" -type f -name '*.ps1' | while IFS= read -r ps1_path; do
  add_utf8_bom "${ps1_path}"
done

CREATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cat > "${PACKAGE_ROOT}/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "package_version": "${PACKAGE_VERSION}",
  "environment": "test",
  "created_at": "${CREATED_AT}",
  "npm_package": "xparse-cli@${NPM_VERSION}",
  "npm_registry": "${NPM_REGISTRY}",
  "source": {"commit": "${SKILLS_COMMIT}", "dirty": ${SKILLS_DIRTY}},
  "supported_installers": ["macos", "linux", "windows"],
  "connector_assets_are_local": true,
  "dllf_required": false,
  "command_line_launcher": false,
  "network_required_for_cli_install": true
}
MANIFEST

(
  cd "${PACKAGE_ROOT}"
  export LC_ALL=C
  find . -type f ! -name SHA256SUMS | sort | while IFS= read -r file; do
    ${SHA256_COMMAND} "${file}"
  done > SHA256SUMS
)
(
  cd "${TEMP_ROOT}"
  zip -qry "${PACKAGE_PATH}" "${PACKAGE_NAME}"
)

printf '%s\n' "${PACKAGE_PATH}"
