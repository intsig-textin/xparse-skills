#!/bin/sh
set -eu

VERSION="${XPARSER_VERSION:-v2.1.0-workbuddy-test.1}"
DOWNLOAD_BASE="${XPARSER_DOWNLOAD_BASE:-https://dllf.intsig.net/download/2026/Solution/xparse-cli}"
CONNECTOR_DIR="${WORKBUDDY_CONNECTOR_DIR:-${HOME}/.workbuddy/connectors-marketplace/connectors/textin-xparse}"
CONNECTOR_FILE="${CONNECTOR_DIR}/cli.json"
CONNECTOR_BACKUP="${CONNECTOR_FILE}.production.bak"
PROFILE_DIR="${XPARSE_WORKBUDDY_PROFILE_DIR:-${HOME}/.xparse-cli/profiles/workbuddy}"
PROFILE_BACKUP="${PROFILE_DIR}.production.bak"
DOWNLOAD_FILE="${CONNECTOR_FILE}.download"

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -f "${DOWNLOAD_FILE}" ]; then
    rm -f "${DOWNLOAD_FILE}"
  fi
}
trap cleanup EXIT

if [ ! -f "${CONNECTOR_FILE}" ]; then
  fail "未找到 TextIn xParse 的 WorkBuddy 配置：${CONNECTOR_FILE}。请先安装并启动一次 WorkBuddy。"
fi

mkdir -p "${CONNECTOR_DIR}"
if [ -n "${XPARSE_TEST_CONNECTOR_FILE:-}" ]; then
  cp "${XPARSE_TEST_CONNECTOR_FILE}" "${DOWNLOAD_FILE}"
else
  CONNECTOR_URL="${XPARSE_TEST_CONNECTOR_URL:-${DOWNLOAD_BASE}/${VERSION}/workbuddy-cli.json}"
  printf '正在下载 WorkBuddy 测试配置：%s\n' "${CONNECTOR_URL}"
  curl -fsSL "${CONNECTOR_URL}" -o "${DOWNLOAD_FILE}"
fi

grep -q '"XPARSE_OAUTH_CLIENT_ID"[[:space:]]*:[[:space:]]*"cli_textin_xparse_workbuddy"' \
  "${DOWNLOAD_FILE}" ||
  fail "下载的文件不是预期的 WorkBuddy 测试配置。"
grep -q '"authUrlDomain"[[:space:]]*:[[:space:]]*"textin-sandbox.intsig.com"' \
  "${DOWNLOAD_FILE}" ||
  fail "下载的配置没有指向 TextIn 测试环境。"
if grep -q '/latest/' "${DOWNLOAD_FILE}"; then
  fail "测试配置不能引用 latest 目录。"
fi

if grep -q '"XPARSE_OAUTH_CLIENT_ID"[[:space:]]*:[[:space:]]*"cli_textin_xparse_workbuddy"' \
  "${CONNECTOR_FILE}"; then
  mv "${DOWNLOAD_FILE}" "${CONNECTOR_FILE}"
  printf 'WorkBuddy 已在使用 TextIn 测试配置，配置已刷新。\n'
else
  if [ -e "${CONNECTOR_BACKUP}" ]; then
    fail "检测到未恢复的生产配置备份：${CONNECTOR_BACKUP}。请先运行恢复脚本。"
  fi
  if [ -d "${PROFILE_DIR}" ] && [ -e "${PROFILE_BACKUP}" ]; then
    fail "检测到未恢复的 WorkBuddy profile 备份：${PROFILE_BACKUP}。请先运行恢复脚本。"
  fi

  mv "${CONNECTOR_FILE}" "${CONNECTOR_BACKUP}"
  if [ -d "${PROFILE_DIR}" ]; then
    mv "${PROFILE_DIR}" "${PROFILE_BACKUP}"
  fi
  mv "${DOWNLOAD_FILE}" "${CONNECTOR_FILE}"
  printf '已备份生产配置，并切换到 TextIn 测试环境。\n'
fi

printf '\n请完全退出并重新打开 WorkBuddy，然后在 TextIn xParse 中点击“连接”。\n'
printf 'WorkBuddy 将自动安装指定测试版 CLI，并打开 TextIn 测试环境授权页。\n'
printf '测试结束后请运行 restore-workbuddy-production.sh 恢复生产配置和原登录态。\n'
