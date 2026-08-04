#!/bin/sh
set -eu

VERSION="${XPARSER_VERSION:-v2.1.0-workbuddy-pre.1}"
DOWNLOAD_BASE="${XPARSER_DOWNLOAD_BASE:-https://dllf.intsig.net/download/2026/Solution/xparse-cli}"
MARKETPLACE_ROOT="${WORKBUDDY_MARKETPLACE_ROOT:-${HOME}/.workbuddy/connectors-marketplace}"
CATALOG_FILE="${WORKBUDDY_CONNECTOR_CATALOG:-${MARKETPLACE_ROOT}/.codebuddy-connector/connectors.json}"
CONNECTORS_DIR="${WORKBUDDY_CONNECTORS_DIR:-${MARKETPLACE_ROOT}/connectors}"
CONNECTOR_DIR="${WORKBUDDY_CONNECTOR_DIR:-${CONNECTORS_DIR}/textin-xparse}"
CONNECTOR_BACKUP="${CONNECTOR_DIR}.production.bak"
MARKETPLACE_ICONS_DIR="${WORKBUDDY_MARKETPLACE_ICONS_DIR:-${MARKETPLACE_ROOT}/icons}"
MARKETPLACE_ICON="${MARKETPLACE_ICONS_DIR}/textin-xparse.png"
MARKETPLACE_ICON_BACKUP="${MARKETPLACE_ICON}.production.bak"
CATALOG_BACKUP="${CATALOG_FILE}.textin-xparse.production.bak"
PROFILE_DIR="${XPARSE_WORKBUDDY_PROFILE_DIR:-${HOME}/.xparse-cli/profiles/workbuddy}"
PROFILE_BACKUP="${PROFILE_DIR}.production.bak"
CLI_PATH="${XPARSE_CLI_PATH:-${HOME}/.local/bin/xparse-cli}"
CLI_BACKUP="${CLI_PATH}.production.bak"
ACTIVE_SKILLS_DIR="${WORKBUDDY_CONNECTOR_SKILLS_DIR:-${HOME}/.workbuddy/connectors/skills/connector-textin-xparse}"
ACTIVE_SKILLS_BACKUP="${ACTIVE_SKILLS_DIR}.production.bak"
STAGE_DIR="${CONNECTORS_DIR}/.textin-xparse.test-download.$$"
CATALOG_DOWNLOAD="${CATALOG_FILE}.textin-xparse-test.download.$$"
ENTRY_DOWNLOAD="${CATALOG_FILE}.textin-xparse-entry.download.$$"
MARKER_FILE="${CONNECTOR_DIR}/.workbuddy-test"
EXPECTED_ICON_SHA256="2f5159dd77b1d4625d44ab5ac30a4b40417be1280083ae4c86ea674d485c4234"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

cleanup() {
  for path in "${CATALOG_DOWNLOAD}" "${ENTRY_DOWNLOAD}"; do
    if [ -f "${path}" ]; then
      rm -f "${path}"
    fi
  done
  if [ -d "${STAGE_DIR}" ]; then
    find "${STAGE_DIR}" -depth -delete 2>/dev/null || true
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
fetch_asset "workbuddy-xparse-parse.zip" "${STAGE_DIR}/xparse-parse.zip"

command -v unzip >/dev/null 2>&1 ||
  fail "缺少 unzip，无法安装 Connector Skill。"
mkdir "${STAGE_DIR}/skills"
unzip -q "${STAGE_DIR}/xparse-parse.zip" -d "${STAGE_DIR}/skills" ||
  fail "无法解压 Connector Skill。"
rm -f "${STAGE_DIR}/xparse-parse.zip"

grep -q '"XPARSE_OAUTH_CLIENT_ID"[[:space:]]*:[[:space:]]*"cli_textin_xparse_workbuddy"' \
  "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置不是预期的 WorkBuddy pre 配置。"
grep -q '"authUrlDomain"[[:space:]]*:[[:space:]]*"textin-api-pre.intsig.com"' \
  "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置没有指向 TextIn pre 环境。"
grep -q '"source"[[:space:]]*:[[:space:]]*"textin-xparse"' \
  "${STAGE_DIR}/connector-meta.json" ||
  fail "下载的 Connector 元数据无效。"
grep -q '"id"[[:space:]]*:[[:space:]]*"textin-xparse"' "${ENTRY_DOWNLOAD}" ||
  fail "下载的 marketplace 注册项无效。"
if grep -q '/latest/' "${STAGE_DIR}/cli.json"; then
  fail "pre 配置不能引用 latest 目录。"
fi
if command -v sha256sum >/dev/null 2>&1; then
  ICON_SHA256="$(sha256sum "${STAGE_DIR}/icon.png" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ICON_SHA256="$(shasum -a 256 "${STAGE_DIR}/icon.png" | awk '{print $1}')"
else
  fail "缺少 sha256sum 或 shasum，无法校验 Connector 图标。"
fi
if [ "${ICON_SHA256}" != "${EXPECTED_ICON_SHA256}" ]; then
  fail "下载的 Connector 图标不是预期的 TextIn xParse Logo。"
fi
if [ ! -f "${STAGE_DIR}/skills/xparse-parse/SKILL.md" ]; then
  fail "下载的 Connector Skill 不完整：缺少 xparse-parse/SKILL.md。"
fi
if [ -e "${STAGE_DIR}/skills/xparse-doc-tools" ]; then
  fail "pre Connector 只能包含 xparse-parse Skill。"
fi

printf '%s\n' "${VERSION}" > "${STAGE_DIR}/.workbuddy-test"

if [ -f "${MARKER_FILE}" ]; then
  OLD_SKILLS="${CONNECTOR_DIR}/skills.test-refresh.$$"
  ACTIVE_REFRESH="${ACTIVE_SKILLS_DIR}.test-refresh.$$"
  if [ -e "${OLD_SKILLS}" ]; then
    fail "Connector Skill 刷新暂存目录已存在：${OLD_SKILLS}。"
  fi
  if [ -e "${ACTIVE_REFRESH}" ]; then
    fail "已激活 Skill 刷新暂存目录已存在：${ACTIVE_REFRESH}。"
  fi
  if [ ! -e "${ACTIVE_SKILLS_BACKUP}" ] && [ -d "${ACTIVE_SKILLS_DIR}" ]; then
    # Migration from test packages that predate activated-Skill backup support.
    # Preserve the only existing copy so the restore script can put it back.
    mv "${ACTIVE_SKILLS_DIR}" "${ACTIVE_SKILLS_BACKUP}"
  fi
  if [ ! -e "${MARKETPLACE_ICON_BACKUP}" ] && [ -f "${MARKETPLACE_ICON}" ]; then
    mv "${MARKETPLACE_ICON}" "${MARKETPLACE_ICON_BACKUP}"
  fi
  mv "${STAGE_DIR}/cli.json" "${CONNECTOR_DIR}/cli.json"
  mv "${STAGE_DIR}/connector-meta.json" "${CONNECTOR_DIR}/connector-meta.json"
  mv "${STAGE_DIR}/icon.png" "${CONNECTOR_DIR}/icon.png"
  mkdir -p "${MARKETPLACE_ICONS_DIR}"
  cp "${CONNECTOR_DIR}/icon.png" "${MARKETPLACE_ICON}"
  if [ -d "${CONNECTOR_DIR}/skills" ]; then
    mv "${CONNECTOR_DIR}/skills" "${OLD_SKILLS}"
  fi
  mv "${STAGE_DIR}/skills" "${CONNECTOR_DIR}/skills"
  if [ -d "${OLD_SKILLS}" ]; then
    find "${OLD_SKILLS}" -depth -delete
  fi
  if [ -d "${ACTIVE_SKILLS_DIR}" ]; then
    mv "${ACTIVE_SKILLS_DIR}" "${ACTIVE_REFRESH}"
  fi
  mkdir -p "$(dirname "${ACTIVE_SKILLS_DIR}")"
  cp -R "${CONNECTOR_DIR}/skills" "${ACTIVE_SKILLS_DIR}"
  if [ -d "${ACTIVE_REFRESH}" ]; then
    find "${ACTIVE_REFRESH}" -depth -delete
  fi
  mv "${STAGE_DIR}/.workbuddy-test" "${MARKER_FILE}"
  printf 'WorkBuddy 已安装 TextIn xParse pre Connector，文件已刷新。\n'
else
  for backup in \
    "${CATALOG_BACKUP}" \
    "${CONNECTOR_BACKUP}" \
    "${MARKETPLACE_ICON_BACKUP}" \
    "${PROFILE_BACKUP}" \
    "${CLI_BACKUP}" \
    "${ACTIVE_SKILLS_BACKUP}"; do
    if [ -e "${backup}" ]; then
      fail "检测到未恢复的备份：${backup}。请先运行恢复脚本。"
    fi
  done

  if grep -q '"id"[[:space:]]*:[[:space:]]*"textin-xparse"' "${CATALOG_FILE}"; then
    cp "${CATALOG_FILE}" "${CATALOG_DOWNLOAD}"
  elif command -v ruby >/dev/null 2>&1; then
    ruby -rjson - "${CATALOG_FILE}" "${ENTRY_DOWNLOAD}" "${CATALOG_DOWNLOAD}" <<'RUBY'
catalog_path, entry_path, output_path = ARGV
catalog = JSON.parse(File.read(catalog_path, encoding: "UTF-8"))
entry = JSON.parse(File.read(entry_path, encoding: "UTF-8"))
connectors = catalog["connectors"]
raise "connectors must be an array" unless connectors.is_a?(Array)
connectors.unshift(entry)
File.write(output_path, JSON.pretty_generate(catalog) + "\n", mode: "w:UTF-8")
RUBY
  elif command -v python3 >/dev/null 2>&1; then
    python3 - "${CATALOG_FILE}" "${ENTRY_DOWNLOAD}" "${CATALOG_DOWNLOAD}" <<'PYTHON'
import json
import sys

catalog_path, entry_path, output_path = sys.argv[1:]
with open(catalog_path, encoding="utf-8") as source:
    catalog = json.load(source)
with open(entry_path, encoding="utf-8") as source:
    entry = json.load(source)
connectors = catalog.get("connectors")
if not isinstance(connectors, list):
    raise ValueError("connectors must be an array")
connectors.insert(0, entry)
with open(output_path, "w", encoding="utf-8") as destination:
    json.dump(catalog, destination, ensure_ascii=False, indent=2)
    destination.write("\n")
PYTHON
  else
    fail "缺少可用的 JSON 解析器；macOS 需要 ruby，Linux 需要 ruby 或 python3。"
  fi

  if command -v plutil >/dev/null 2>&1; then
    plutil -convert xml1 -o /dev/null "${CATALOG_DOWNLOAD}" ||
      fail "更新后的 WorkBuddy Connector 注册表不是有效 JSON。"
  fi

  cp -p "${CATALOG_FILE}" "${CATALOG_BACKUP}"
  if [ -d "${CONNECTOR_DIR}" ]; then
    mv "${CONNECTOR_DIR}" "${CONNECTOR_BACKUP}"
  fi
  if [ -f "${MARKETPLACE_ICON}" ]; then
    mv "${MARKETPLACE_ICON}" "${MARKETPLACE_ICON_BACKUP}"
  fi
  if [ -d "${PROFILE_DIR}" ]; then
    mv "${PROFILE_DIR}" "${PROFILE_BACKUP}"
  fi
  if [ -f "${CLI_PATH}" ]; then
    mv "${CLI_PATH}" "${CLI_BACKUP}"
  fi
  if [ -d "${ACTIVE_SKILLS_DIR}" ]; then
    mv "${ACTIVE_SKILLS_DIR}" "${ACTIVE_SKILLS_BACKUP}"
  fi
  mv "${CATALOG_DOWNLOAD}" "${CATALOG_FILE}"
  mv "${STAGE_DIR}" "${CONNECTOR_DIR}"
  mkdir -p "${MARKETPLACE_ICONS_DIR}"
  cp "${CONNECTOR_DIR}/icon.png" "${MARKETPLACE_ICON}"
  mkdir -p "$(dirname "${ACTIVE_SKILLS_DIR}")"
  cp -R "${CONNECTOR_DIR}/skills" "${ACTIVE_SKILLS_DIR}"
  printf '已注入 TextIn xParse pre Connector，并备份执行前的 WorkBuddy 状态。\n'
fi

printf '\n请完全退出并重新打开 WorkBuddy，然后在 TextIn xParse 中点击“连接”。\n'
printf 'WorkBuddy 将安装唯一的 xparse-parse Skill、指定 pre 版 CLI，并打开 TextIn pre 环境授权页。\n'
printf '测试结束后请运行 restore-workbuddy-production.sh 恢复执行前状态。\n'
