# WorkBuddy 本地测试指南

本文说明如何在本机把 TextIn xParse test Connector 临时注入 WorkBuddy，完成 OAuth、Skill 和解析链路验收，并在测试后恢复原有环境。

## 适用范围

本流程用于 WorkBuddy Connector 的开发和发布前验证，覆盖：

- marketplace 中能够发现 TextIn xParse Connector；
- Connector 自动安装指定的 test 版 CLI 和唯一的 `xparse-parse` Skill；
- Device OAuth 能够完成连接、状态检查和退出；
- WorkBuddy 使用隔离的 `workbuddy` Profile；
- 免费解析默认生效，付费解析只在显式选择后生效；
- 解析请求携带 `X-From: workbuddy`；
- 测试结束后能够恢复测试前的 Connector、CLI、Profile、Skill 和 marketplace 状态。

> [!WARNING]
> 启用脚本会临时替换本机已安装的 TextIn xParse Connector、CLI、`workbuddy` Profile、已激活 Skill 和 marketplace 注册项。不要在未准备恢复脚本时执行启用操作，也不要使用 `latest` 作为测试版本。

## 文件说明

| 文件 | 平台 | 用途 |
|------|------|------|
| `enable-workbuddy-test.sh` | macOS / Linux | 备份正式状态并注入测试 Connector |
| `restore-workbuddy-production.sh` | macOS / Linux | 恢复正式状态并归档测试版本 |
| `enable-workbuddy-test.ps1` | Windows | 备份正式状态并注入测试 Connector |
| `restore-workbuddy-production.ps1` | Windows | 恢复正式状态并归档测试版本 |

脚本默认测试 `v2.2.0-workbuddy-test.3`，并从以下不可变版本目录下载资源：

```text
https://dllf.intsig.net/download/2026/Solution/xparse-cli/<version>/
```

## 前置条件

1. 已安装 WorkBuddy，并至少启动过一次，使 Connector 注册表已经生成。
2. 完全退出 WorkBuddy 后再执行启用或恢复脚本。
3. 测试版本已经发布到独立版本目录；不得指向 `latest`。
4. macOS / Linux 需要 `curl`、`unzip`，以及 `shasum` 或 `sha256sum`。首次注入新 Connector 时还需要 `ruby` 或 `python3` 解析注册表。
5. Windows 使用 PowerShell，并允许当前命令通过 `-ExecutionPolicy Bypass` 运行测试脚本。
6. 准备一个小型、无敏感信息的 PDF 或图片作为验收文件。

启用脚本会校验以下内容，任一不符合都会停止注入：

- OAuth client 是 `cli_textin_xparse_workbuddy`；
- 授权环境是 `textin-sandbox.intsig.com`；
- Connector id/source 是 `textin-xparse`；
- 配置不引用 `/latest/`；
- 图标 SHA256 与仓库版本一致；
- Skill 包包含 `xparse-parse/SKILL.md`，且不包含 `xparse-doc-tools`。

## 1. 先执行代码级验证

在仓库根目录运行：

```bash
cd cli
go test ./...
cd ..
```

需要验证跨平台构建和发布文件时，可使用不可变的 test 版本号：

```bash
cd cli
./build.sh v2.2.0-workbuddy-test.3
DRY_RUN=1 ./publish-version.sh v2.2.0-workbuddy-test.3
cd ..
```

`DRY_RUN=1` 只检查并展示发布清单，不上传文件。真实发布会修改远端下载目录，必须经过单独确认后执行。

正式 2.2.0 分发物使用生产 Connector 配置，并继续写入不可变版本目录：

```bash
cd cli
./build.sh v2.2.0
DRY_RUN=1 ./publish-version.sh v2.2.0 prod
cd ..
```

以上命令只构建并审查生产发布清单；去掉 `DRY_RUN=1` 才会上传，上传前必须单独确认。

## 2. macOS / Linux 注入测试 Connector

在仓库根目录运行：

```bash
XPARSER_VERSION=v2.2.0-workbuddy-test.3 \
  sh connector/test/enable-workbuddy-test.sh
```

脚本会：

1. 下载并校验测试 Connector、marketplace 条目、图标和 Skill；
2. 备份当前 Connector、marketplace 注册表与图标；
3. 备份当前 CLI、`workbuddy` Profile 和已激活 Skill；
4. 注入测试 Connector，并写入 `.workbuddy-test` marker；
5. 如果测试 Connector 已存在，只刷新测试文件，同时保留最初的正式备份。

脚本成功后，完全退出并重新打开 WorkBuddy。

## 3. Windows 注入测试 Connector

在仓库根目录打开 PowerShell：

```powershell
$env:XPARSER_VERSION = "v2.2.0-workbuddy-test.3"
powershell -NoProfile -ExecutionPolicy Bypass `
  -File connector/test/enable-workbuddy-test.ps1
```

脚本成功后，完全退出并重新打开 WorkBuddy。

版本发布后，Windows 测试人员无需下载仓库文件，直接在 PowerShell 执行：

```powershell
irm 'https://dllf.intsig.net/download/2026/Solution/xparse-cli/v2.2.0-workbuddy-test.3/enable-workbuddy.ps1' | iex
```

## 4. 使用仓库中的本地 Connector/Skill 资源

`XPARSE_TEST_ASSET_DIR` 可让启用脚本读取本地 Connector 和 Skill 资源，不从下载站获取这五个文件。macOS / Linux 示例：

```bash
XPARSE_ASSET_DIR="$(mktemp -d)"
cp connector/cli.test.json "$XPARSE_ASSET_DIR/workbuddy-cli.json"
cp connector/connector-meta.json "$XPARSE_ASSET_DIR/workbuddy-connector-meta.json"
cp connector/icon.png "$XPARSE_ASSET_DIR/workbuddy-icon.png"
cp connector/marketplace-entry.json "$XPARSE_ASSET_DIR/workbuddy-marketplace-entry.json"
(
  cd skills
  zip -qr "$XPARSE_ASSET_DIR/workbuddy-xparse-parse.zip" xparse-parse
)

XPARSE_TEST_ASSET_DIR="$XPARSE_ASSET_DIR" \
  XPARSER_VERSION=v2.2.0-workbuddy-test.3 \
  sh connector/test/enable-workbuddy-test.sh
```

该模式用于验证当前仓库里的 Connector 元数据、图标、Skill 和注入/恢复机制。CLI 的安装来源仍由本地 `workbuddy-cli.json` 中的 `init` 命令决定；如果它仍指向已发布的 pre 目录，WorkBuddy 使用的仍是已发布 CLI，而不是当前未发布源码。

测试并恢复完成后，可清理刚创建的临时资源目录：

```bash
find "$XPARSE_ASSET_DIR" -depth -delete
unset XPARSE_ASSET_DIR
```

Windows 使用本地资源时，将同名五个文件放入一个目录，并在运行启用脚本前设置：

```powershell
$env:XPARSE_TEST_ASSET_DIR = "C:\path\to\workbuddy-test-assets"
```

## 5. WorkBuddy 内验收

按照以下顺序验证，并记录失败步骤、WorkBuddy 版本、CLI 版本和请求 ID：

### Connector 和安装

- marketplace 中只出现一个 TextIn xParse Connector 条目；
- Connector 图标和名称正确；
- 点击连接后能够安装 CLI；
- 版本检查显示预期的 `2.2.0` CLI；
- Connector 只激活 `xparse-parse` Skill。

### OAuth

- 点击“连接”后使用 Device Flow 打开 TextIn pre 授权页；
- 页面展示 WorkBuddy 品牌和当前登录账号；
- 同意授权后 Connector 状态变为已连接；
- 状态检查返回 `logged_in: true`、`method: oauth`；
- 断开连接后只清理 `workbuddy` Profile，不影响终端默认 Profile。

必要时可在终端只读检查 WorkBuddy Profile：

```bash
"$HOME/.local/bin/xparse-cli" --profile workbuddy auth status --output=json
"$HOME/.local/bin/xparse-cli" version
```

Windows：

```powershell
& "$env:USERPROFILE\.xparse-cli\bin\xparse-cli.exe" `
  --profile workbuddy auth status --output=json
& "$env:USERPROFILE\.xparse-cli\bin\xparse-cli.exe" version
```

不要打印、复制或记录 OAuth access token、refresh token 或 AppKey。

### 解析

1. 让 WorkBuddy 使用小文件执行免费解析，确认默认使用 `--api free` 并正常返回 Markdown。
2. 在获得用户明确同意后执行付费 OAuth 解析，确认 `--api paid --auth-method oauth` 可用。
3. 确认 WorkBuddy 中的命令都显式使用 `--profile workbuddy`。
4. 通过服务端日志或统计记录确认解析请求的 `X-From` 为 `workbuddy`。OAuth `/oauth21/token` 轮询本身不携带该请求头。
5. 确认免费额度不足时不会自动切换付费接口；必须停止并征得用户同意。

## 6. 恢复测试前状态

测试结束后，无论验收成功或失败，都应执行恢复脚本。

macOS / Linux：

```bash
sh connector/test/restore-workbuddy-production.sh
```

Windows：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File connector/test/restore-workbuddy-production.ps1
```

版本发布后可直接一键恢复：

```powershell
irm 'https://dllf.intsig.net/download/2026/Solution/xparse-cli/v2.2.0-workbuddy-test.3/restore-workbuddy-production.ps1' | iex
```

恢复脚本会：

- 恢复测试前的 marketplace 注册表和图标；
- 恢复测试前的 Connector、CLI、`workbuddy` Profile 和已激活 Skill；
- 将本次测试 Connector 与测试状态按时间戳归档；
- 保留归档供问题排查，不直接删除测试证据。

恢复完成后，再次完全退出并重新打开 WorkBuddy，并检查原有 Connector 状态。

## 7. 安全机制和故障处理

### 提示存在未恢复备份

启用脚本发现 `.production.bak` 时会拒绝覆盖。先运行恢复脚本；不要手工覆盖备份。

### 缺少 marker 但存在孤立备份

macOS / Linux 恢复脚本会自动处理这类未完成事务：

1. 将当前同名 Profile、CLI、Skill 或 Connector 状态归档到
   `.textin-xparse-test-backups/orphan-recovery.<timestamp>`；
2. 将 `.production.bak` 恢复到正式路径，不永久删除唯一的正式备份；
3. 清除对下一次 enable 的备份路径冲突。

恢复成功后可直接重新执行 enable 一键安装命令。macOS、Linux 和
Windows 回滚脚本使用相同的归档后恢复策略。

### WorkBuddy 中没有出现 Connector

确认：

1. WorkBuddy 已完全退出并重启；
2. `~/.workbuddy/connectors-marketplace/.codebuddy-connector/connectors.json` 中存在 `textin-xparse`；
3. Connector 目录中存在 `.workbuddy-test`；
4. 启用脚本没有在资源校验阶段失败。

### OAuth 打开了错误环境

测试配置必须使用 `textin-sandbox.intsig.com` 和 `cli_textin_xparse_workbuddy`。如果不是，停止测试并执行恢复脚本，不要继续授权。

### 刷新测试版本

再次运行启用脚本会刷新现有测试 Connector，但不会覆盖最初保存的正式备份。更换版本时应显式设置 `XPARSER_VERSION`，并确保该版本目录中的 Connector 配置与 CLI 安装版本一致。

## 8. 可覆盖的路径变量

脚本默认使用当前用户目录，也支持通过环境变量在隔离目录中测试：

| 变量 | 用途 |
|------|------|
| `XPARSER_VERSION` | 不可变测试版本号 |
| `XPARSER_DOWNLOAD_BASE` | 测试资源下载根地址 |
| `XPARSE_TEST_ASSET_DIR` | 本地测试资源目录 |
| `WORKBUDDY_MARKETPLACE_ROOT` | WorkBuddy marketplace 根目录 |
| `WORKBUDDY_CONNECTOR_CATALOG` | Connector 注册表路径 |
| `WORKBUDDY_CONNECTORS_DIR` | Connector 集合目录 |
| `WORKBUDDY_CONNECTOR_DIR` | TextIn xParse Connector 目录 |
| `WORKBUDDY_MARKETPLACE_ICONS_DIR` | marketplace 图标目录 |
| `WORKBUDDY_CONNECTOR_SKILLS_DIR` | WorkBuddy 已激活 Skill 目录 |
| `XPARSE_WORKBUDDY_PROFILE_DIR` | WorkBuddy CLI Profile 目录 |
| `XPARSE_CLI_PATH` | CLI 可执行文件路径 |
| `WORKBUDDY_TEST_BACKUP_ROOT` | 恢复时的测试归档目录 |

在自定义路径测试时，启用和恢复必须使用完全相同的一组环境变量。
