#!/bin/bash

# 用法: ./deploy.sh 版本号
# 示例: ./deploy.sh v1.0.0

# 检查是否传入版本参数
if [ $# -ne 1 ]; then
    echo "错误：请传入版本号！"
    echo "用法：$0 <version>"
    echo "示例：$0 v1.0.0"
    exit 1
fi

# 接收命令行传入的版本号
VERSION="$1"

# 基础配置
DIST_DIR="dist"
BASE_PATH="xparse-cli"

# 上传通用安装脚本
/bin/bash upload.sh install/install.sh "${BASE_PATH}/install.sh"
/bin/bash upload.sh install/install.ps1 "${BASE_PATH}/install.ps1"

# 上传对应版本的安装脚本
/bin/bash upload.sh install/install.sh "${BASE_PATH}/${VERSION}/install.sh"
/bin/bash upload.sh install/install.ps1 "${BASE_PATH}/${VERSION}/install.ps1"

# 所有二进制文件列表
BINARY_FILES=(
  "xparse-cli-darwin-amd64"
  "xparse-cli-darwin-arm64"
  "xparse-cli-linux-amd64"
  "xparse-cli-linux-arm64"
  "xparse-cli-windows-amd64.exe"
  "xparse-cli-windows-arm64.exe"
)

# 批量上传到指定版本目录
for file in "${BINARY_FILES[@]}"; do
  /bin/bash upload.sh "${DIST_DIR}/${file}" "${BASE_PATH}/${VERSION}/${file}"
done

# 批量上传到 latest 最新目录
for file in "${BINARY_FILES[@]}"; do
  /bin/bash upload.sh "${DIST_DIR}/${file}" "${BASE_PATH}/latest/${file}"
done

echo "✅ 版本 ${VERSION} 上传完成！"