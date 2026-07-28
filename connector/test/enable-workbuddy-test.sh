#!/bin/sh
set -eu

VERSION="${XPARSER_VERSION:-v2.1.0-workbuddy-test.2}"
DOWNLOAD_BASE="${XPARSER_DOWNLOAD_BASE:-https://dllf.intsig.net/download/2026/Solution/xparse-cli}"
MARKETPLACE_ROOT="${WORKBUDDY_MARKETPLACE_ROOT:-${HOME}/.workbuddy/connectors-marketplace}"
CATALOG_FILE="${WORKBUDDY_CONNECTOR_CATALOG:-${MARKETPLACE_ROOT}/.codebuddy-connector/connectors.json}"
CONNECTORS_DIR="${WORKBUDDY_CONNECTORS_DIR:-${MARKETPLACE_ROOT}/connectors}"
CONNECTOR_DIR="${WORKBUDDY_CONNECTOR_DIR:-${CONNECTORS_DIR}/textin-xparse}"
CONNECTOR_BACKUP="${CONNECTOR_DIR}.production.bak"
CATALOG_BACKUP="${CATALOG_FILE}.textin-xparse.production.bak"
PROFILE_DIR="${XPARSE_WORKBUDDY_PROFILE_DIR:-${HOME}/.xparse-cli/profiles/workbuddy}"
PROFILE_BACKUP="${PROFILE_DIR}.production.bak"
CLI_PATH="${XPARSE_CLI_PATH:-${HOME}/.local/bin/xparse-cli}"
CLI_BACKUP="${CLI_PATH}.production.bak"
STAGE_DIR="${CONNECTORS_DIR}/.textin-xparse.test-download.$$"
CATALOG_DOWNLOAD="${CATALOG_FILE}.textin-xparse-test.download.$$"
ENTRY_DOWNLOAD="${CATALOG_FILE}.textin-xparse-entry.download.$$"
MARKER_FILE="${CONNECTOR_DIR}/.workbuddy-test"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

cleanup() {
  for path in \
    "${STAGE_DIR}/cli.json" \
    "${STAGE_DIR}/connector-meta.json" \
    "${STAGE_DIR}/icon.png" \
    "${STAGE_DIR}/.workbuddy-test" \
    "${CATALOG_DOWNLOAD}" \
    "${ENTRY_DOWNLOAD}"; do
    if [ -f "${path}" ]; then
      rm -f "${path}"
    fi
  done
  if [ -d "${STAGE_DIR}" ]; then
    rmdir "${STAGE_DIR}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

fetch_asset() {
  remote_name="$1"
  local_name="$2"
  if [ -n "${XPARSE_TEST_ASSET_DIR:-}" ]; then
    cp "${XPARSE_TEST_ASSET_DIR}/${remote_name}" "${local_name}"
  else
    asset_url="${DOWNLOAD_BASE}/${VERSION}/${remote_name}"
    printf '正在下载：%s\n' "${asset_url}"
    curl -fsSL "${asset_url}" -o "${local_name}"
  fi
}

if [ ! -f "${CATALOG_FILE}" ]; then
  fail "未找到 WorkBuddy Connector 注册表：${CATALOG_FILE}。请先安装并启动一次 WorkBuddy。"
fi
mkdir -p "${CONNECTORS_DIR}"
mkdir "${STAGE_DIR}"

fetch_asset "workbuddy-cli.json" "${STAGE_DIR}/cli.json"
fetch_asset "workbuddy-connector-meta.json" "${STAGE_DIR}/connector-meta.json"
fetch_asset "workbuddy-icon.png" "${STAGE_DIR}/icon.png"
fetch_asset "workbuddy-marketplace-entry.json" "${ENTRY_DOWNLOAD}"

grep -q '"XPARSE_OAUTH_CLIENT_ID"[[:space:]]*:[[:space:]]*"cli_textin_xparse_workbuddy"' \
  "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置不是预期的 WorkBuddy 测试配置。"
grep -q '"authUrlDomain"[[:space:]]*:[[:space:]]*"textin-sandbox.intsig.com"' \
  "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置没有指向 TextIn 测试环境。"
grep -q '"source"[[:space:]]*:[[:space:]]*"textin-xparse"' \
  "${STAGE_DIR}/connector-meta.json" ||
  fail "下载的 Connector 元数据无效。"
grep -q '"id"[[:space:]]*:[[:space:]]*"textin-xparse"' "${ENTRY_DOWNLOAD}" ||
  fail "下载的 marketplace 注册项无效。"
if grep -q '/latest/' "${STAGE_DIR}/cli.json"; then
  fail "测试配置不能引用 latest 目录。"
fi
if [ ! -s "${STAGE_DIR}/icon.png" ]; then
  fail "下载的 Connector 图标为空。"
fi

printf '%s\n' "${VERSION}" > "${STAGE_DIR}/.workbuddy-test"

if [ -f "${MARKER_FILE}" ]; then
  mv "${STAGE_DIR}/cli.json" "${CONNECTOR_DIR}/cli.json"
  mv "${STAGE_DIR}/connector-meta.json" "${CONNECTOR_DIR}/connector-meta.json"
  mv "${STAGE_DIR}/icon.png" "${CONNECTOR_DIR}/icon.png"
  mv "${STAGE_DIR}/.workbuddy-test" "${MARKER_FILE}"
  printf 'WorkBuddy 已安装 TextIn xParse 测试 Connector，文件已刷新。\n'
else
  for backup in \
    "${CATALOG_BACKUP}" \
    "${CONNECTOR_BACKUP}" \
    "${PROFILE_BACKUP}" \
    "${CLI_BACKUP}"; do
    if [ -e "${backup}" ]; then
      fail "检测到未恢复的备份：${backup}。请先运行恢复脚本。"
    fi
  done

  if grep -q '"id"[[:space:]]*:[[:space:]]*"textin-xparse"' "${CATALOG_FILE}"; then
    cp "${CATALOG_FILE}" "${CATALOG_DOWNLOAD}"
  else
    entry_json="$(tr -d '\n' < "${ENTRY_DOWNLOAD}")"
    awk -v entry="${entry_json}" '
      /^[[:space:]]*"connectors"[[:space:]]*:[[:space:]]*\[/ && !inserted {
        print
        print "    " entry ","
        inserted = 1
        next
      }
      { print }
      END {
        if (!inserted) {
          exit 42
        }
      }
    ' "${CATALOG_FILE}" > "${CATALOG_DOWNLOAD}" ||
      fail "无法向 WorkBuddy Connector 注册表加入 TextIn xParse。"
  fi

  if command -v plutil >/dev/null 2>&1; then
    plutil -convert xml1 -o /dev/null "${CATALOG_DOWNLOAD}" ||
      fail "更新后的 WorkBuddy Connector 注册表不是有效 JSON。"
  fi

  cp -p "${CATALOG_FILE}" "${CATALOG_BACKUP}"
  if [ -d "${CONNECTOR_DIR}" ]; then
    mv "${CONNECTOR_DIR}" "${CONNECTOR_BACKUP}"
  fi
  if [ -d "${PROFILE_DIR}" ]; then
    mv "${PROFILE_DIR}" "${PROFILE_BACKUP}"
  fi
  if [ -f "${CLI_PATH}" ]; then
    mv "${CLI_PATH}" "${CLI_BACKUP}"
  fi
  mv "${CATALOG_DOWNLOAD}" "${CATALOG_FILE}"
  mv "${STAGE_DIR}" "${CONNECTOR_DIR}"
  printf '已注入 TextIn xParse 测试 Connector，并备份执行前的 WorkBuddy 状态。\n'
fi

printf '\n请完全退出并重新打开 WorkBuddy，然后在 TextIn xParse 中点击“连接”。\n'
printf 'WorkBuddy 将自动安装指定测试版 CLI，并打开 TextIn 测试环境授权页。\n'
printf '测试结束后请运行 restore-workbuddy-production.sh 恢复执行前状态。\n'
