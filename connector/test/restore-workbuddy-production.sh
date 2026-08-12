#!/bin/sh
set -eu

MARKETPLACE_ROOT="${WORKBUDDY_MARKETPLACE_ROOT:-${HOME}/.workbuddy/connectors-marketplace}"
PRODUCT_ROOT="${WORKBUDDY_PRODUCT_ROOT:-$(dirname "${MARKETPLACE_ROOT}")}"
ACCOUNT_STATE_ROOT="${WORKBUDDY_ACCOUNT_STATE_ROOT:-${PRODUCT_ROOT}/connectors}"
ACCOUNT_STATE_BACKUP_SUFFIX=".textin-xparse.production.bak"
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
ACTIVE_SKILLS_DIR="${WORKBUDDY_CONNECTOR_SKILLS_DIR:-${PRODUCT_ROOT}/connectors/skills/connector-textin-xparse}"
ACTIVE_SKILLS_BACKUP="${ACTIVE_SKILLS_DIR}.production.bak"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"
TEST_BACKUP_ROOT="${WORKBUDDY_TEST_BACKUP_ROOT:-${MARKETPLACE_ROOT}/.textin-xparse-test-backups}"
CONNECTOR_TEST_BACKUP="${TEST_BACKUP_ROOT}/connector.${TIMESTAMP}"
CATALOG_TEST_BACKUP="${TEST_BACKUP_ROOT}/connectors.${TIMESTAMP}.json"
PROFILE_TEST_BACKUP="${PROFILE_DIR}.test.${TIMESTAMP}.bak"
CLI_TEST_BACKUP="${CLI_PATH}.test.${TIMESTAMP}.bak"
ACTIVE_SKILLS_TEST_BACKUP="${TEST_BACKUP_ROOT}/activated-skills.${TIMESTAMP}"
MARKETPLACE_ICON_TEST_BACKUP="${TEST_BACKUP_ROOT}/marketplace-icon.${TIMESTAMP}.png"
ORPHAN_RECOVERY_ROOT="${TEST_BACKUP_ROOT}/orphan-recovery.${TIMESTAMP}"
ACCOUNT_STATE_BACKUP_LIST="${CATALOG_FILE}.textin-xparse-account-backups.$$"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

cleanup() {
  rm -f "${ACCOUNT_STATE_BACKUP_LIST}"
}
trap cleanup EXIT

discover_account_state_backups() {
  : > "${ACCOUNT_STATE_BACKUP_LIST}"
  if [ -d "${ACCOUNT_STATE_ROOT}" ]; then
    find "${ACCOUNT_STATE_ROOT}" -type f \
      -name "connector-states.v3.json${ACCOUNT_STATE_BACKUP_SUFFIX}" \
      -print > "${ACCOUNT_STATE_BACKUP_LIST}"
  fi
}

restore_account_state_backups() {
  archive_root="$1"
  archive_prefix="$2"
  counter=0
  while IFS= read -r backup_path; do
    counter=$((counter + 1))
    archive_path="${archive_root}/${archive_prefix}.${counter}.json"
    if [ -e "${archive_path}" ]; then
      fail "账号状态归档目标已存在：${archive_path}。请稍后重试。"
    fi
  done < "${ACCOUNT_STATE_BACKUP_LIST}"

  counter=0
  while IFS= read -r backup_path; do
    counter=$((counter + 1))
    state_path="${backup_path%${ACCOUNT_STATE_BACKUP_SUFFIX}}"
    archive_path="${archive_root}/${archive_prefix}.${counter}.json"
    if [ -f "${state_path}" ]; then
      mkdir -p "${archive_root}"
      mv "${state_path}" "${archive_path}"
    fi
    mkdir -p "$(dirname "${state_path}")"
    mv "${backup_path}" "${state_path}"
    printf '已恢复账号状态：%s\n' "${state_path}"
  done < "${ACCOUNT_STATE_BACKUP_LIST}"
}

restore_orphaned_backup() {
  current_path="$1"
  backup_path="$2"
  archive_name="$3"
  if [ ! -e "${backup_path}" ]; then
    return
  fi
  if [ -e "${current_path}" ]; then
    mv "${current_path}" "${ORPHAN_RECOVERY_ROOT}/${archive_name}"
  fi
  mkdir -p "$(dirname "${current_path}")"
  mv "${backup_path}" "${current_path}"
  printf '已恢复孤立正式备份：%s\n' "${current_path}"
}

discover_account_state_backups

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
  if [ -s "${ACCOUNT_STATE_BACKUP_LIST}" ]; then
    ORPHANED_BACKUPS="${ORPHANED_BACKUPS}
  - 账号状态备份（${ACCOUNT_STATE_ROOT}）"
  fi
  if [ -n "${ORPHANED_BACKUPS}" ]; then
    if [ -e "${ORPHAN_RECOVERY_ROOT}" ]; then
      fail "孤立备份恢复目录已存在：${ORPHAN_RECOVERY_ROOT}。请稍后重试。"
    fi
    mkdir -p "${ORPHAN_RECOVERY_ROOT}"
    printf '未找到测试 Connector marker，开始自动恢复孤立正式备份：%s\n' \
      "${ORPHANED_BACKUPS}"
    restore_orphaned_backup \
      "${CATALOG_FILE}" "${CATALOG_BACKUP}" "connectors.current.json"
    restore_orphaned_backup \
      "${CONNECTOR_DIR}" "${CONNECTOR_BACKUP}" "connector.current"
    restore_orphaned_backup \
      "${MARKETPLACE_ICON}" "${MARKETPLACE_ICON_BACKUP}" "marketplace-icon.current.png"
    restore_orphaned_backup \
      "${PROFILE_DIR}" "${PROFILE_BACKUP}" "profile.current"
    restore_orphaned_backup \
      "${CLI_PATH}" "${CLI_BACKUP}" "xparse-cli.current"
    restore_orphaned_backup \
      "${ACTIVE_SKILLS_DIR}" "${ACTIVE_SKILLS_BACKUP}" "activated-skills.current"
    restore_account_state_backups "${ORPHAN_RECOVERY_ROOT}" "account-state.current"
    printf '已自动恢复孤立备份，原当前状态已归档到：%s\n' \
      "${ORPHAN_RECOVERY_ROOT}"
    printf '现在可以重新运行 enable 一键安装命令。\n'
    exit 0
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
restore_account_state_backups "${TEST_BACKUP_ROOT}" "account-state.${TIMESTAMP}"
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

printf '已恢复执行测试脚本前的 WorkBuddy marketplace、Connector、账号、CLI、profile 和已激活 Skill 状态。\n'
printf '本次测试 Connector 已归档到：%s\n' "${CONNECTOR_TEST_BACKUP}"
printf '\n请完全退出并重新打开 WorkBuddy，使恢复后的状态生效。\n'
