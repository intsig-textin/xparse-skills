#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 2 ]; then
  echo "Usage: $0 upload_file destination [content_type]"
  exit 1
fi

upload_file="$1"
dst_url="$2"
content_type="${3:-}"
azcopy_bin="${AZCOPY_BIN:-$(command -v azcopy || true)}"

if [ -z "${azcopy_bin}" ]; then
  echo "[ERROR] azcopy not found; install it or set AZCOPY_BIN." >&2
  exit 1
fi

# 上传
all_dst_url="https://ccidownload.blob.core.chinacloudapi.cn/download/2026/Solution/${dst_url}?sv=2024-11-04&ss=bfqt&srt=co&sp=rwdlacup&se=2028-01-08T17:42:09Z&st=2026-01-08T09:27:09Z&spr=https,http&sig=wvpL80PES5pnMWOsUjg%2FZEp1dUowIgb%2FX31jaedSIsw%3D"
echo "[INFO] Uploading via azcopy to: https://dllf.intsig.net/download/2026/Solution/${dst_url}"
azcopy_args=(copy "${upload_file}" "${all_dst_url}")
if [ -n "${content_type}" ]; then
  azcopy_args+=("--content-type=${content_type}")
fi
"${azcopy_bin}" "${azcopy_args[@]}"

# 计算 md5
if command -v md5sum >/dev/null 2>&1; then
  md5_value="$(md5sum "${upload_file}" | awk '{print $1}')"
elif command -v md5 >/dev/null 2>&1; then
  md5_value="$(md5 -q "${upload_file}")"
else
  echo "[ERROR] md5sum or md5 not found." >&2
  exit 1
fi

# 打印信息
echo "============================"
echo "[下载链接] https://dllf.intsig.net/download/2026/Solution/${dst_url}"
echo "[md5] ${md5_value}"
echo "============================"
