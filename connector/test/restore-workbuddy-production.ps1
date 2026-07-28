$ErrorActionPreference = "Stop"

$UserHome = [Environment]::GetFolderPath("UserProfile")
$ConnectorDir = if ($env:WORKBUDDY_CONNECTOR_DIR) {
    $env:WORKBUDDY_CONNECTOR_DIR
} else {
    Join-Path $UserHome ".workbuddy\connectors-marketplace\connectors\textin-xparse"
}
$ConnectorFile = Join-Path $ConnectorDir "cli.json"
$ConnectorBackup = "${ConnectorFile}.production.bak"
$ProfileDir = if ($env:XPARSE_WORKBUDDY_PROFILE_DIR) {
    $env:XPARSE_WORKBUDDY_PROFILE_DIR
} else {
    Join-Path $UserHome ".xparse-cli\profiles\workbuddy"
}
$ProfileBackup = "${ProfileDir}.production.bak"
$Timestamp = Get-Date -Format "yyyyMMddHHmmss"

if (-not (Test-Path -LiteralPath $ConnectorFile -PathType Leaf)) {
    throw "未找到 TextIn xParse 的 WorkBuddy 配置：${ConnectorFile}。"
}

$CurrentConfig = Get-Content -LiteralPath $ConnectorFile -Raw | ConvertFrom-Json
if ($CurrentConfig.env.XPARSE_OAUTH_CLIENT_ID -ne "cli_textin_xparse_workbuddy") {
    Write-Host "当前已经不是 TextIn 测试配置，无需恢复。"
    exit 0
}
if (-not (Test-Path -LiteralPath $ConnectorBackup -PathType Leaf)) {
    throw "未找到生产配置备份：${ConnectorBackup}。为避免覆盖现有配置，已停止恢复。"
}

$ConnectorTestBackup = "${ConnectorFile}.test.${Timestamp}.bak"
$ProfileTestBackup = "${ProfileDir}.test.${Timestamp}.bak"
if ((Test-Path -LiteralPath $ConnectorTestBackup) -or
    (Test-Path -LiteralPath $ProfileTestBackup)) {
    throw "本次恢复的备份目标已存在，请稍后重试。"
}

Move-Item -LiteralPath $ConnectorFile -Destination $ConnectorTestBackup
Move-Item -LiteralPath $ConnectorBackup -Destination $ConnectorFile

if (Test-Path -LiteralPath $ProfileDir -PathType Container) {
    Move-Item -LiteralPath $ProfileDir -Destination $ProfileTestBackup
}
if (Test-Path -LiteralPath $ProfileBackup -PathType Container) {
    Move-Item -LiteralPath $ProfileBackup -Destination $ProfileDir
}

Write-Host "已恢复 WorkBuddy 的 TextIn xParse 生产配置和原登录态。"
Write-Host "测试 profile 保留在：${ProfileTestBackup}"
Write-Host ""
Write-Host "请完全退出并重新打开 WorkBuddy，使恢复后的配置生效。"
