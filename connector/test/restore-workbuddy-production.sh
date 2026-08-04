#!/bin/sh
set -eu

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
TIMESTAMP="$(date +%Y%m%d%H%M%S)"
TEST_BACKUP_ROOT="${WORKBUDDY_TEST_BACKUP_ROOT:-${MARKETPLACE_ROOT}/.textin-xparse-test-backups}"
CONNECTOR_TEST_BACKUP="${TEST_BACKUP_ROOT}/connector.${TIMESTAMP}"
CATALOG_TEST_BACKUP="${TEST_BACKUP_ROOT}/connectors.${TIMESTAMP}.json"
PROFILE_TEST_BACKUP="${PROFILE_DIR}.test.${TIMESTAMP}.bak"
CLI_TEST_BACKUP="${CLI_PATH}.test.${TIMESTAMP}.bak"
ACTIVE_SKILLS_TEST_BACKUP="${TEST_BACKUP_ROOT}/activated-skills.${TIMESTAMP}"
MARKETPLACE_ICON_TEST_BACKUP="${TEST_BACKUP_ROOT}/marketplace-icon.${TIMESTAMP}.png"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

if [ ! -f "${CONNECTOR_DIR}/.workbuddy-test" ]; then
  ORPHANED_BACKUPS=""
  for backup in \
    "${CATALOG_BACKUP}" \
    "${CONNECTOR_BACKUP}" \
    "${MARKETPLACE_ICON_BACKUP}" \
    "${PROFILE_BACKUP}" \
    "${CLI_BACKUP}" \
    "${ACTIVE_SKILLS_BACKUP}"; do
    if [ -e "${backup}" ]; then
      ORPHANED_BACKUPS="${ORPHANED_BACKUPS}
  - ${backup}"
    fi
  done
  if [ -n "${ORPHANED_BACKUPS}" ]; then
    fail "未找到测试 Connector marker，但检测到孤立备份：${ORPHANED_BACKUPS}
这些备份可能来自未完成的旧测试事务，无法自动判断是否应恢复。请先归档或人工确认后再运行 enable。"
  fi
  printf '当前未安装 TextIn xParse 测试 Connector，且没有遗留备份，无需恢复。\n'
  exit 0
fi
if [ ! -f "${CATALOG_BACKUP}" ]; then
  fail "未找到 WorkBuddy Connector 注册表备份：${CATALOG_BACKUP}。为避免覆盖现有状态，已停止恢复。"
fi

for target in \
  "${CONNECTOR_TEST_BACKUP}" \
  "${CATALOG_TEST_BACKUP}" \
  "${MARKETPLACE_ICON_TEST_BACKUP}" \
  "${PROFILE_TEST_BACKUP}" \
  "${CLI_TEST_BACKUP}" \
  "${ACTIVE_SKILLS_TEST_BACKUP}"; do
  if [ -e "${target}" ]; then
    fail "本次恢复的归档目标已存在：${target}。请稍后重试。"
  fi
done

mkdir -p "${TEST_BACKUP_ROOT}"
mv "${CONNECTOR_DIR}" "${CONNECTOR_TEST_BACKUP}"
mv "${CATALOG_FILE}" "${CATALOG_TEST_BACKUP}"
mv "${CATALOG_BACKUP}" "${CATALOG_FILE}"
if [ -f "${MARKETPLACE_ICON}" ]; then
  mv "${MARKETPLACE_ICON}" "${MARKETPLACE_ICON_TEST_BACKUP}"
fi
if [ -f "${MARKETPLACE_ICON_BACKUP}" ]; then
  mkdir -p "${MARKETPLACE_ICONS_DIR}"
  mv "${MARKETPLACE_ICON_BACKUP}" "${MARKETPLACE_ICON}"
fi

if [ -d "${CONNECTOR_BACKUP}" ]; then
  mv "${CONNECTOR_BACKUP}" "${CONNECTOR_DIR}"
fi
if [ -d "${PROFILE_DIR}" ]; then
  mv "${PROFILE_DIR}" "${PROFILE_TEST_BACKUP}"
fi
if [ -d "${PROFILE_BACKUP}" ]; then
  mv "${PROFILE_BACKUP}" "${PROFILE_DIR}"
fi
if [ -f "${CLI_PATH}" ]; then
  mv "${CLI_PATH}" "${CLI_TEST_BACKUP}"
fi
if [ -f "${CLI_BACKUP}" ]; then
  mv "${CLI_BACKUP}" "${CLI_PATH}"
fi
if [ -d "${ACTIVE_SKILLS_DIR}" ]; then
  mv "${ACTIVE_SKILLS_DIR}" "${ACTIVE_SKILLS_TEST_BACKUP}"
fi
if [ -d "${ACTIVE_SKILLS_BACKUP}" ]; then
  mkdir -p "$(dirname "${ACTIVE_SKILLS_DIR}")"
  mv "${ACTIVE_SKILLS_BACKUP}" "${ACTIVE_SKILLS_DIR}"
fi

printf '已恢复执行测试脚本前的 WorkBuddy marketplace、Connector、CLI、profile 和已激活 Skill 状态。\n'
printf '本次测试 Connector 已归档到：%s\n' "${CONNECTOR_TEST_BACKUP}"
printf '\n请完全退出并重新打开 WorkBuddy，使恢复后的状态生效。\n'
