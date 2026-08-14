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
NPM_PREFIX="${XPARSE_NPM_PREFIX:-${HOME}/.xparse-cli/npm}"
NPM_BACKUP="${NPM_PREFIX}.production.bak"
COMMAND_DIR="${XPARSE_COMMAND_DIR:-${HOME}/.local/bin}"
COMMAND_PATH="${COMMAND_DIR}/xparse-cli"
COMMAND_BACKUP="${COMMAND_PATH}.production.bak"
COMMAND_MARKER="${COMMAND_PATH}.workbuddy-npm"
PATH_MARKER="${XPARSE_PATH_MARKER:-${HOME}/.xparse-cli/workbuddy-command-path.added}"
ACTIVE_SKILLS_DIR="${WORKBUDDY_CONNECTOR_SKILLS_DIR:-${HOME}/.workbuddy/connectors/skills/connector-textin-xparse}"
ACTIVE_SKILLS_BACKUP="${ACTIVE_SKILLS_DIR}.production.bak"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"
TEST_BACKUP_ROOT="${WORKBUDDY_TEST_BACKUP_ROOT:-${MARKETPLACE_ROOT}/.textin-xparse-test-backups}"
CONNECTOR_TEST_BACKUP="${TEST_BACKUP_ROOT}/connector.${TIMESTAMP}"
CATALOG_TEST_BACKUP="${TEST_BACKUP_ROOT}/connectors.${TIMESTAMP}.json"
PROFILE_TEST_BACKUP="${PROFILE_DIR}.test.${TIMESTAMP}.bak"
NPM_TEST_BACKUP="${NPM_PREFIX}.test.${TIMESTAMP}.bak"
ACTIVE_SKILLS_TEST_BACKUP="${TEST_BACKUP_ROOT}/activated-skills.${TIMESTAMP}"
MARKETPLACE_ICON_TEST_BACKUP="${TEST_BACKUP_ROOT}/marketplace-icon.${TIMESTAMP}.png"
ORPHAN_RECOVERY_ROOT="${TEST_BACKUP_ROOT}/orphan-recovery.${TIMESTAMP}"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
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

restore_command_launcher() {
  if [ -f "${COMMAND_MARKER}" ]; then
    rm -f "${COMMAND_PATH}" "${COMMAND_MARKER}"
  fi
  if [ -e "${COMMAND_BACKUP}" ] || [ -L "${COMMAND_BACKUP}" ]; then
    mkdir -p "${COMMAND_DIR}"
    if [ -e "${COMMAND_PATH}" ] || [ -L "${COMMAND_PATH}" ]; then
      rm -f "${COMMAND_PATH}"
    fi
    mv "${COMMAND_BACKUP}" "${COMMAND_PATH}"
  fi
  if [ -f "${PATH_MARKER}" ]; then
    shell_profile="$(sed -n '1p' "${PATH_MARKER}")"
    path_line="$(sed -n '2p' "${PATH_MARKER}")"
    if [ -n "${shell_profile}" ] && [ -f "${shell_profile}" ]; then
      profile_temp="${shell_profile}.xparse-path-restore.$$"
      cp -p "${shell_profile}" "${profile_temp}"
      awk -v comment='# Added by xparse-cli WorkBuddy npm launcher' \
        -v line="${path_line}" '$0 != comment && $0 != line' \
        "${shell_profile}" > "${profile_temp}"
      mv "${profile_temp}" "${shell_profile}"
    fi
    rm -f "${PATH_MARKER}"
  fi
}

if [ ! -f "${CONNECTOR_DIR}/.workbuddy-test" ]; then
  ORPHANED_BACKUPS=""
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
      ORPHANED_BACKUPS="${ORPHANED_BACKUPS}
  - ${backup}"
    fi
  done
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
      "${NPM_PREFIX}" "${NPM_BACKUP}" "npm-cli.current"
    restore_command_launcher
    restore_orphaned_backup \
      "${ACTIVE_SKILLS_DIR}" "${ACTIVE_SKILLS_BACKUP}" "activated-skills.current"
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
  "${NPM_TEST_BACKUP}" \
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
if [ -d "${NPM_BACKUP}" ]; then
  if [ -d "${NPM_PREFIX}" ]; then
    mv "${NPM_PREFIX}" "${NPM_TEST_BACKUP}"
  fi
  mv "${NPM_BACKUP}" "${NPM_PREFIX}"
fi
restore_command_launcher
if [ -d "${ACTIVE_SKILLS_DIR}" ]; then
  mv "${ACTIVE_SKILLS_DIR}" "${ACTIVE_SKILLS_TEST_BACKUP}"
fi
if [ -d "${ACTIVE_SKILLS_BACKUP}" ]; then
  mkdir -p "$(dirname "${ACTIVE_SKILLS_DIR}")"
  mv "${ACTIVE_SKILLS_BACKUP}" "${ACTIVE_SKILLS_DIR}"
fi

printf '已恢复执行测试脚本前的 WorkBuddy marketplace、Connector、npm CLI、profile 和已激活 Skill 状态。\n'
printf '本次测试 Connector 已归档到：%s\n' "${CONNECTOR_TEST_BACKUP}"
printf '\n请完全退出并重新打开 WorkBuddy，使恢复后的状态生效。\n'
