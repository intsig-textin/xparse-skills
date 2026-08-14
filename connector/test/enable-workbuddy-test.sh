#!/bin/sh
set -eu

VERSION="${XPARSER_VERSION:-v2.2.0-workbuddy-test.3}"
NPM_VERSION="${XPARSE_NPM_VERSION:-2.2.1-beta.2}"
NPM_REGISTRY="${XPARSE_NPM_REGISTRY:-https://registry.npmmirror.com}"
INSTALL_CLI_WITH_LOCAL_ASSETS="${XPARSE_INSTALL_CLI_WITH_LOCAL_ASSETS:-0}"
EXPECTED_AUTH_DOMAIN="${XPARSE_EXPECTED_AUTH_DOMAIN:-textin-sandbox.intsig.com}"
PROFILE_BASE_URL="${XPARSE_PROFILE_BASE_URL:-https://textin-sandbox.intsig.com}"
ENVIRONMENT_LABEL="${XPARSE_ENVIRONMENT_LABEL:-test}"
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
NPM_PREFIX="${XPARSE_NPM_PREFIX:-${HOME}/.xparse-cli/npm}"
NPM_BACKUP="${NPM_PREFIX}.production.bak"
CLI_PATH="${XPARSE_CLI_PATH:-${NPM_PREFIX}/bin/xparse-cli}"
COMMAND_DIR="${XPARSE_COMMAND_DIR:-${HOME}/.local/bin}"
COMMAND_PATH="${COMMAND_DIR}/xparse-cli"
COMMAND_BACKUP="${COMMAND_PATH}.production.bak"
COMMAND_MARKER="${COMMAND_PATH}.workbuddy-npm"
PATH_MARKER="${XPARSE_PATH_MARKER:-${HOME}/.xparse-cli/workbuddy-command-path.added}"
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

install_command_launcher() {
  mkdir -p "${COMMAND_DIR}"
  if [ ! -f "${COMMAND_MARKER}" ]; then
    if [ -e "${COMMAND_BACKUP}" ] || [ -L "${COMMAND_BACKUP}" ]; then
      fail "检测到未恢复的命令行入口备份：${COMMAND_BACKUP}。请先运行恢复脚本。"
    fi
    if [ -e "${COMMAND_PATH}" ] || [ -L "${COMMAND_PATH}" ]; then
      mv "${COMMAND_PATH}" "${COMMAND_BACKUP}"
    fi
  elif [ -e "${COMMAND_PATH}" ] || [ -L "${COMMAND_PATH}" ]; then
    rm -f "${COMMAND_PATH}"
  fi
  ln -s "${CLI_PATH}" "${COMMAND_PATH}"
  printf '%s\n' "npm:${NPM_VERSION}" > "${COMMAND_MARKER}"

  case ":${PATH}:" in
    *:"${COMMAND_DIR}":*) path_needs_persist=0 ;;
    *) path_needs_persist=1 ;;
  esac
  if [ "${path_needs_persist}" = "1" ]; then
    export PATH="${COMMAND_DIR}:${PATH}"
    if [ ! -f "${PATH_MARKER}" ]; then
      case "$(basename "${SHELL:-sh}")" in
        zsh) shell_profile="${HOME}/.zshrc" ;;
        bash) shell_profile="${HOME}/.bashrc" ;;
        fish) shell_profile="${HOME}/.config/fish/config.fish" ;;
        *) shell_profile="${HOME}/.profile" ;;
      esac
      mkdir -p "$(dirname "${shell_profile}")" "$(dirname "${PATH_MARKER}")"
      if [ "$(basename "${SHELL:-sh}")" = "fish" ]; then
        path_line="set -gx PATH \"${COMMAND_DIR}\" \$PATH"
      else
        path_line="export PATH=\"${COMMAND_DIR}:\$PATH\""
      fi
      printf '\n# Added by xparse-cli WorkBuddy npm launcher\n%s\n' "${path_line}" >> "${shell_profile}"
      printf '%s\n%s\n' "${shell_profile}" "${path_line}" > "${PATH_MARKER}"
    fi
  fi
  env -u NODE_OPTIONS "${COMMAND_PATH}" version >/dev/null 2>&1 ||
    fail "命令行入口无法启动 xparse-cli。请执行恢复脚本。"
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
  fail "下载的 CLI 配置不是预期的 WorkBuddy test 配置。"
grep -q "\"authUrlDomain\"[[:space:]]*:[[:space:]]*\"${EXPECTED_AUTH_DOMAIN}\"" \
  "${STAGE_DIR}/cli.json" ||
  fail "下载的 CLI 配置没有指向 TextIn ${ENVIRONMENT_LABEL} 环境。"
grep -q '"source"[[:space:]]*:[[:space:]]*"textin-xparse"' \
  "${STAGE_DIR}/connector-meta.json" ||
  fail "下载的 Connector 元数据无效。"
grep -q '"id"[[:space:]]*:[[:space:]]*"textin-xparse"' "${ENTRY_DOWNLOAD}" ||
  fail "下载的 marketplace 注册项无效。"
if grep -q '/latest/' "${STAGE_DIR}/cli.json"; then
  fail "${ENVIRONMENT_LABEL} 配置不能引用 latest 目录。"
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
  fail "test Connector 只能包含 xparse-parse Skill。"
fi

printf '%s\n' "${VERSION}" > "${STAGE_DIR}/.workbuddy-test"

if command -v ruby >/dev/null 2>&1; then
  ruby -rjson - "${CATALOG_FILE}" "${ENTRY_DOWNLOAD}" "${CATALOG_DOWNLOAD}" <<'RUBY'
catalog_path, entry_path, output_path = ARGV
catalog = JSON.parse(File.read(catalog_path, encoding: "UTF-8"))
entry = JSON.parse(File.read(entry_path, encoding: "UTF-8"))
connectors = catalog["connectors"]
raise "connectors must be an array" unless connectors.is_a?(Array)
catalog["connectors"] = [entry] + connectors.reject { |item| item["id"] == entry["id"] }
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
catalog["connectors"] = [entry] + [
    item for item in connectors if item.get("id") != entry["id"]
]
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
  mv "${CATALOG_DOWNLOAD}" "${CATALOG_FILE}"
  printf 'WorkBuddy 已安装 TextIn xParse test Connector，文件已刷新。\n'
else
  for backup in \
    "${CATALOG_BACKUP}" \
    "${CONNECTOR_BACKUP}" \
    "${MARKETPLACE_ICON_BACKUP}" \
    "${PROFILE_BACKUP}" \
    "${NPM_BACKUP}" \
    "${COMMAND_BACKUP}" \
    "${COMMAND_MARKER}" \
    "${PATH_MARKER}" \
    "${ACTIVE_SKILLS_BACKUP}"; do
    if [ -e "${backup}" ]; then
      fail "检测到未恢复的备份：${backup}。请先运行恢复脚本。"
    fi
  done

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
  if { [ -z "${XPARSE_TEST_ASSET_DIR:-}" ] || [ "${INSTALL_CLI_WITH_LOCAL_ASSETS}" = "1" ]; } &&
    [ -d "${NPM_PREFIX}" ]; then
    mv "${NPM_PREFIX}" "${NPM_BACKUP}"
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
  printf '已注入 TextIn xParse test Connector，并备份执行前的 WorkBuddy 状态。\n'
fi

if [ -z "${XPARSE_TEST_ASSET_DIR:-}" ] || [ "${INSTALL_CLI_WITH_LOCAL_ASSETS}" = "1" ]; then
  command -v node >/dev/null 2>&1 ||
    fail "缺少 Node.js 18 或更高版本，无法安装 npm CLI。"
  command -v npm >/dev/null 2>&1 ||
    fail "缺少 npm，无法安装 xparse-cli。"
  NODE_MAJOR="$(env -u NODE_OPTIONS node -p "process.versions.node.split('.')[0]")"
  if [ "${NODE_MAJOR}" -lt 18 ]; then
    fail "Node.js 版本过低（${NODE_MAJOR}），xparse-cli 需要 Node.js 18 或更高版本。"
  fi
  printf '正在通过 npm 安装 %s CLI：xparse-cli@%s（%s）\n' \
    "${ENVIRONMENT_LABEL}" "${NPM_VERSION}" "${NPM_REGISTRY}"
  env -u NODE_OPTIONS npm install --global --prefix "${NPM_PREFIX}" \
    "--registry=${NPM_REGISTRY}" "xparse-cli@${NPM_VERSION}" ||
    fail "npm 安装 xparse-cli@${NPM_VERSION} 失败。请执行恢复脚本。"
  if [ ! -x "${CLI_PATH}" ]; then
    fail "CLI 安装失败：未找到 ${CLI_PATH}。请执行恢复脚本。"
  fi
  env -u NODE_OPTIONS "${CLI_PATH}" --profile workbuddy \
    config set base_url "${PROFILE_BASE_URL}" ||
    fail "CLI 无法写入 ${ENVIRONMENT_LABEL} Profile。请执行恢复脚本。"
  install_command_launcher
fi

printf '\n请完全退出并重新打开 WorkBuddy，然后在 TextIn xParse 中点击“连接”。\n'
printf 'WorkBuddy 将安装唯一的 xparse-parse Skill、npm CLI %s，并打开 TextIn %s 环境授权页。\n' "${NPM_VERSION}" "${ENVIRONMENT_LABEL}"
printf '新终端可直接运行 xparse-cli；如当前终端尚未生效，请重新打开终端。\n'
printf '测试结束后请运行 restore-workbuddy-production.sh 恢复执行前状态。\n'
