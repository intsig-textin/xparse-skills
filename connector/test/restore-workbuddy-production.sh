#!/bin/sh
set -eu

CONNECTOR_DIR="${WORKBUDDY_CONNECTOR_DIR:-${HOME}/.workbuddy/connectors-marketplace/connectors/textin-xparse}"
CONNECTOR_FILE="${CONNECTOR_DIR}/cli.json"
CONNECTOR_BACKUP="${CONNECTOR_FILE}.production.bak"
PROFILE_DIR="${XPARSE_WORKBUDDY_PROFILE_DIR:-${HOME}/.xparse-cli/profiles/workbuddy}"
PROFILE_BACKUP="${PROFILE_DIR}.production.bak"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

if [ ! -f "${CONNECTOR_FILE}" ]; then
  fail "未找到 TextIn xParse 的 WorkBuddy 配置：${CONNECTOR_FILE}。"
fi

if ! grep -q '"XPARSE_OAUTH_CLIENT_ID"[[:space:]]*:[[:space:]]*"cli_textin_xparse_workbuddy"' \
  "${CONNECTOR_FILE}"; then
  printf '当前已经不是 TextIn 测试配置，无需恢复。\n'
  exit 0
fi
if [ ! -f "${CONNECTOR_BACKUP}" ]; then
  fail "未找到生产配置备份：${CONNECTOR_BACKUP}。为避免覆盖现有配置，已停止恢复。"
fi

CONNECTOR_TEST_BACKUP="${CONNECTOR_FILE}.test.${TIMESTAMP}.bak"
PROFILE_TEST_BACKUP="${PROFILE_DIR}.test.${TIMESTAMP}.bak"
if [ -e "${CONNECTOR_TEST_BACKUP}" ] || [ -e "${PROFILE_TEST_BACKUP}" ]; then
  fail "本次恢复的备份目标已存在，请稍后重试。"
fi

mv "${CONNECTOR_FILE}" "${CONNECTOR_TEST_BACKUP}"
mv "${CONNECTOR_BACKUP}" "${CONNECTOR_FILE}"

if [ -d "${PROFILE_DIR}" ]; then
  mv "${PROFILE_DIR}" "${PROFILE_TEST_BACKUP}"
fi
if [ -d "${PROFILE_BACKUP}" ]; then
  mv "${PROFILE_BACKUP}" "${PROFILE_DIR}"
fi

printf '已恢复 WorkBuddy 的 TextIn xParse 生产配置和原登录态。\n'
printf '测试 profile 保留在：%s\n' "${PROFILE_TEST_BACKUP}"
printf '\n请完全退出并重新打开 WorkBuddy，使恢复后的配置生效。\n'
