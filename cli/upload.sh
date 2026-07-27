#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 upload_file"
  exit 1
fi

upload_file="$1"
dst_url="$2"

# 上传
all_dst_url="https://ccidownload.blob.core.chinacloudapi.cn/download/2026/Solution/${dst_url}?sv=2024-11-04&ss=bfqt&srt=co&sp=rwdlacup&se=2028-01-08T17:42:09Z&st=2026-01-08T09:27:09Z&spr=https,http&sig=wvpL80PES5pnMWOsUjg%2FZEp1dUowIgb%2FX31jaedSIsw%3D"
echo "[INFO] Uploading via azcopy to: ${all_dst_url}"
/data/home/haibo_jiang/azcopy_linux_amd64_10.30.1/azcopy copy "${upload_file}" "${all_dst_url}"

# 计算 md5
md5_value="$(md5sum "${upload_file}" | awk '{print $1}')"

# 打印信息
echo "============================"
echo "[下载链接] https://dllf.intsig.net/download/2026/Solution/${dst_url}"
echo "[md5] ${md5_value}"
echo "============================"