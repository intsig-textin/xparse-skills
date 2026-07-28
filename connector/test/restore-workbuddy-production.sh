#!/bin/sh
set -eu

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
TIMESTAMP="$(date +%Y%m%d%H%M%S)"
TEST_BACKUP_ROOT="${WORKBUDDY_TEST_BACKUP_ROOT:-${MARKETPLACE_ROOT}/.textin-xparse-test-backups}"
CONNECTOR_TEST_BACKUP="${TEST_BACKUP_ROOT}/connector.${TIMESTAMP}"
CATALOG_TEST_BACKUP="${TEST_BACKUP_ROOT}/connectors.${TIMESTAMP}.json"
PROFILE_TEST_BACKUP="${PROFILE_DIR}.test.${TIMESTAMP}.bak"
CLI_TEST_BACKUP="${CLI_PATH}.test.${TIMESTAMP}.bak"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

if [ ! -f "${CONNECTOR_DIR}/.workbuddy-test" ]; then
  printf '当前未安装 TextIn xParse 测试 Connector，无需恢复。\n'
  exit 0
fi
if [ ! -f "${CATALOG_BACKUP}" ]; then
  fail "未找到 WorkBuddy Connector 注册表备份：${CATALOG_BACKUP}。为避免覆盖现有状态，已停止恢复。"
fi

for target in \
  "${CONNECTOR_TEST_BACKUP}" \
  "${CATALOG_TEST_BACKUP}" \
  "${PROFILE_TEST_BACKUP}" \
  "${CLI_TEST_BACKUP}"; do
  if [ -e "${target}" ]; then
    fail "本次恢复的归档目标已存在：${target}。请稍后重试。"
  fi
done

mkdir -p "${TEST_BACKUP_ROOT}"
mv "${CONNECTOR_DIR}" "${CONNECTOR_TEST_BACKUP}"
mv "${CATALOG_FILE}" "${CATALOG_TEST_BACKUP}"
mv "${CATALOG_BACKUP}" "${CATALOG_FILE}"

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

printf '已恢复执行测试脚本前的 WorkBuddy marketplace、Connector、CLI 和 profile 状态。\n'
printf '本次测试 Connector 已归档到：%s\n' "${CONNECTOR_TEST_BACKUP}"
printf '\n请完全退出并重新打开 WorkBuddy，使恢复后的状态生效。\n'
