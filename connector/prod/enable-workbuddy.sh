#!/bin/sh
set -eu

VERSION="${XPARSER_VERSION:-v2.1.0}"
DOWNLOAD_BASE="${XPARSER_DOWNLOAD_BASE:-https://dllf.intsig.net/download/2026/Solution/xparse-cli}"
MARKETPLACE_ROOT="${WORKBUDDY_MARKETPLACE_ROOT:-${HOME}/.workbuddy/connectors-marketplace}"
CATALOG_FILE="${WORKBUDDY_CONNECTOR_CATALOG:-${MARKETPLACE_ROOT}/.codebuddy-connector/connectors.json}"
CONNECTORS_DIR="${WORKBUDDY_CONNECTORS_DIR:-${MARKETPLACE_ROOT}/connectors}"
CONNECTOR_DIR="${WORKBUDDY_CONNECTOR_DIR:-${CONNECTORS_DIR}/textin-xparse}"
MARKETPLACE_ICONS_DIR="${WORKBUDDY_MARKETPLACE_ICONS_DIR:-${MARKETPLACE_ROOT}/icons}"
MARKETPLACE_ICON="${MARKETPLACE_ICONS_DIR}/textin-xparse.png"
ACTIVE_SKILLS_DIR="${WORKBUDDY_CONNECTOR_SKILLS_DIR:-${HOME}/.workbuddy/connectors/skills/connector-textin-xparse}"
CLI_PATH="${XPARSE_CLI_PATH:-${HOME}/.local/bin/xparse-cli}"
STAGE_DIR="${CONNECTORS_DIR}/.textin-xparse.prod-download.$$"
CATALOG_DOWNLOAD="${CATALOG_FILE}.textin-xparse-prod.download.$$"
ENTRY_DOWNLOAD="${CATALOG_FILE}.textin-xparse-entry.download.$$"
OLD_CONNECTOR="${CONNECTOR_DIR}.prod-install-old.$$"
OLD_MARKETPLACE_ICON="${MARKETPLACE_ICON}.prod-install-old.$$"
OLD_ACTIVE_SKILLS="${ACTIVE_SKILLS_DIR}.prod-install-old.$$"
OLD_CATALOG="${CATALOG_FILE}.textin-xparse-prod-install-old.$$"
EXPECTED_ICON_SHA256="2f5159dd77b1d4625d44ab5ac30a4b40417be1280083ae4c86ea674d485c4234"

SUCCESS=0
CONNECTOR_INSTALLED=0
ICON_INSTALLED=0
ACTIVE_SKILLS_INSTALLED=0
CATALOG_INSTALLED=0
PROFILE_CONFIGURED=0

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

remove_tree() {
  path="$1"
  if [ -d "${path}" ]; then
    find "${path}" -depth -delete 2>/dev/null || true
  fi
}

cleanup() {
  if [ "${SUCCESS}" -eq 0 ]; then
    if [ "${CATALOG_INSTALLED}" -eq 1 ] && [ -f "${OLD_CATALOG}" ]; then
      cp -p "${OLD_CATALOG}" "${CATALOG_FILE}" 2>/dev/null || true
    fi
    if [ "${ACTIVE_SKILLS_INSTALLED}" -eq 1 ]; then
      remove_tree "${ACTIVE_SKILLS_DIR}"
      if [ -d "${OLD_ACTIVE_SKILLS}" ]; then
        mv "${OLD_ACTIVE_SKILLS}" "${ACTIVE_SKILLS_DIR}" 2>/dev/null || true
      fi
    fi
    if [ "${ICON_INSTALLED}" -eq 1 ]; then
      rm -f "${MARKETPLACE_ICON}"
      if [ -f "${OLD_MARKETPLACE_ICON}" ]; then
        mv "${OLD_MARKETPLACE_ICON}" "${MARKETPLACE_ICON}" 2>/dev/null || true
      fi
    fi
    if [ "${CONNECTOR_INSTALLED}" -eq 1 ]; then
      remove_tree "${CONNECTOR_DIR}"
      if [ -d "${OLD_CONNECTOR}" ]; then
        mv "${OLD_CONNECTOR}" "${CONNECTOR_DIR}" 2>/dev/null || true
      fi
    fi
  fi

  rm -f "${CATALOG_DOWNLOAD}" "${ENTRY_DOWNLOAD}" "${OLD_CATALOG}"
  remove_tree "${STAGE_DIR}"
  if [ "${SUCCESS}" -eq 1 ]; then
    remove_tree "${OLD_CONNECTOR}"
    remove_tree "${OLD_ACTIVE_SKILLS}"
    rm -f "${OLD_MARKETPLACE_ICON}"
  fi
}
trap cleanup EXIT

fetch_asset() {
  remote_name="$1"
  local_name="$2"
  if [ -n "${XPARSE_PROD_ASSET_DIR:-}" ]; then
    cp "${XPARSE_PROD_ASSET_DIR}/${remote_name}" "${local_name}"
  else
    asset_url="${DOWNLOAD_BASE}/${VERSION}/${remote_name}"
    printf '正在下载：%s\n' "${asset_url}"
    curl -fsSL "${asset_url}" -o "${local_name}"
  fi
}

if [ ! -f "${CATALOG_FILE}" ]; then
  fail "未找到 WorkBuddy Connector 注册表：${CATALOG_FILE}。请先安装并启动一次 WorkBuddy。"
fi

for unresolved in \
  "${CONNECTOR_DIR}/.workbuddy-test" \
  "${CONNECTOR_DIR}.production.bak" \
  "${MARKETPLACE_ICON}.production.bak" \
  "${CATALOG_FILE}.textin-xparse.production.bak" \
  "${HOME}/.xparse-cli/profiles/workbuddy.production.bak" \
  "${HOME}/.local/bin/xparse-cli.production.bak" \
  "${ACTIVE_SKILLS_DIR}.production.bak"; do
  if [ -e "${unresolved}" ]; then
    fail "检测到尚未恢复的 WorkBuddy 测试状态：${unresolved}。请先运行对应测试包的恢复脚本。"
  fi
done

for temporary in \
  "${STAGE_DIR}" \
  "${CATALOG_DOWNLOAD}" \
  "${ENTRY_DOWNLOAD}" \
  "${OLD_CONNECTOR}" \
  "${OLD_MARKETPLACE_ICON}" \
  "${OLD_ACTIVE_SKILLS}" \
  "${OLD_CATALOG}"; do
  if [ -e "${temporary}" ]; then
    fail "安装暂存路径已存在：${temporary}。请稍后重试。"
  fi
done

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
  fail "下载的 CLI 配置没有使用 WorkBuddy OAuth client。"
grep -q '"authUrlDomain"[[:space:]]*:[[:space:]]*"api.textin.com"' \
  "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置没有指向 TextIn 正式环境。"
grep -q '"XPARSE_BASE_URL"[[:space:]]*:[[:space:]]*"https://api.textin.com"' \
  "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置没有强制使用 TextIn 正式 API。"
grep -q "/${VERSION}/install.sh" "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置没有固定到 ${VERSION}。"
if grep -q 'textin-api-pre\.intsig\.com\|/latest/' "${STAGE_DIR}/cli.json"; then
  fail "正式 Connector 配置包含 pre 或 latest 引用。"
fi
grep -q '"source"[[:space:]]*:[[:space:]]*"textin-xparse"' \
  "${STAGE_DIR}/connector-meta.json" ||
  fail "下载的 Connector 元数据无效。"
grep -q '"id"[[:space:]]*:[[:space:]]*"textin-xparse"' "${ENTRY_DOWNLOAD}" ||
  fail "下载的 marketplace 注册项无效。"

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
  fail "正式 Connector 只能包含 xparse-parse Skill。"
fi

if command -v ruby >/dev/null 2>&1; then
  ruby -rjson - "${CATALOG_FILE}" "${ENTRY_DOWNLOAD}" "${CATALOG_DOWNLOAD}" <<'RUBY'
catalog_path, entry_path, output_path = ARGV
catalog = JSON.parse(File.read(catalog_path, encoding: "UTF-8"))
entry = JSON.parse(File.read(entry_path, encoding: "UTF-8"))
connectors = catalog["connectors"]
raise "connectors must be an array" unless connectors.is_a?(Array)
connectors.reject! { |item| item.is_a?(Hash) && item["id"] == entry["id"] }
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
catalog["connectors"] = [
    entry,
    *[
        item
        for item in connectors
        if not isinstance(item, dict) or item.get("id") != entry.get("id")
    ],
]
with open(output_path, "w", encoding="utf-8") as destination:
    json.dump(catalog, destination, ensure_ascii=False, indent=2)
    destination.write("\n")
PYTHON
else
  fail "缺少可用的 JSON 解析器；macOS 需要 ruby，Linux 需要 ruby 或 python3。"
fi

cp -p "${CATALOG_FILE}" "${OLD_CATALOG}"
if [ -d "${CONNECTOR_DIR}" ]; then
  mv "${CONNECTOR_DIR}" "${OLD_CONNECTOR}"
fi
mv "${STAGE_DIR}" "${CONNECTOR_DIR}"
CONNECTOR_INSTALLED=1

mkdir -p "${MARKETPLACE_ICONS_DIR}"
if [ -f "${MARKETPLACE_ICON}" ]; then
  mv "${MARKETPLACE_ICON}" "${OLD_MARKETPLACE_ICON}"
fi
cp "${CONNECTOR_DIR}/icon.png" "${MARKETPLACE_ICON}"
ICON_INSTALLED=1

if [ -d "${ACTIVE_SKILLS_DIR}" ]; then
  mv "${ACTIVE_SKILLS_DIR}" "${OLD_ACTIVE_SKILLS}"
fi
mkdir -p "$(dirname "${ACTIVE_SKILLS_DIR}")"
cp -R "${CONNECTOR_DIR}/skills" "${ACTIVE_SKILLS_DIR}"
ACTIVE_SKILLS_INSTALLED=1

mv "${CATALOG_DOWNLOAD}" "${CATALOG_FILE}"
CATALOG_INSTALLED=1

if [ -x "${CLI_PATH}" ]; then
  "${CLI_PATH}" --profile workbuddy config set base_url https://api.textin.com ||
    fail "无法把现有 WorkBuddy CLI profile 切换到 TextIn 正式环境。"
  PROFILE_CONFIGURED=1
fi
SUCCESS=1

printf '已安装 TextIn xParse 正式 Connector（%s）。\n' "${VERSION}"
if [ "${PROFILE_CONFIGURED}" -eq 1 ]; then
  printf '已将现有 WorkBuddy CLI profile 切换到 TextIn 正式环境，登录凭证保持不变。\n'
else
  printf '尚未检测到 CLI；WorkBuddy 首次连接安装 CLI 后会自动写入正式环境配置。\n'
fi
printf '\n请完全退出并重新打开 WorkBuddy，然后在“TextIn xParse·智能文档解析”中点击“连接”。\n'
