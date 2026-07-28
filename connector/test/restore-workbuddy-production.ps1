$ErrorActionPreference = "Stop"

$UserHome = [Environment]::GetFolderPath("UserProfile")
$MarketplaceRoot = if ($env:WORKBUDDY_MARKETPLACE_ROOT) {
    $env:WORKBUDDY_MARKETPLACE_ROOT
} else {
    Join-Path $UserHome ".workbuddy\connectors-marketplace"
}
$CatalogFile = if ($env:WORKBUDDY_CONNECTOR_CATALOG) {
    $env:WORKBUDDY_CONNECTOR_CATALOG
} else {
    Join-Path $MarketplaceRoot ".codebuddy-connector\connectors.json"
}
$ConnectorsDir = if ($env:WORKBUDDY_CONNECTORS_DIR) {
    $env:WORKBUDDY_CONNECTORS_DIR
} else {
    Join-Path $MarketplaceRoot "connectors"
}
$ConnectorDir = if ($env:WORKBUDDY_CONNECTOR_DIR) {
    $env:WORKBUDDY_CONNECTOR_DIR
} else {
    Join-Path $ConnectorsDir "textin-xparse"
}
$ConnectorBackup = "${ConnectorDir}.production.bak"
$CatalogBackup = "${CatalogFile}.textin-xparse.production.bak"
$ProfileDir = if ($env:XPARSE_WORKBUDDY_PROFILE_DIR) {
    $env:XPARSE_WORKBUDDY_PROFILE_DIR
} else {
    Join-Path $UserHome ".xparse-cli\profiles\workbuddy"
}
$ProfileBackup = "${ProfileDir}.production.bak"
$CLIPath = if ($env:XPARSE_CLI_PATH) {
    $env:XPARSE_CLI_PATH
} else {
    Join-Path $UserHome ".xparse-cli\bin\xparse-cli.exe"
}
$CLIBackup = "${CLIPath}.production.bak"
$Timestamp = Get-Date -Format "yyyyMMddHHmmss"
$TestBackupRoot = if ($env:WORKBUDDY_TEST_BACKUP_ROOT) {
    $env:WORKBUDDY_TEST_BACKUP_ROOT
} else {
    Join-Path $MarketplaceRoot ".textin-xparse-test-backups"
}
$ConnectorTestBackup = Join-Path $TestBackupRoot "connector.${Timestamp}"
$CatalogTestBackup = Join-Path $TestBackupRoot "connectors.${Timestamp}.json"
$ProfileTestBackup = "${ProfileDir}.test.${Timestamp}.bak"
$CLITestBackup = "${CLIPath}.test.${Timestamp}.bak"
$MarkerFile = Join-Path $ConnectorDir ".workbuddy-test"

if (-not (Test-Path -LiteralPath $MarkerFile -PathType Leaf)) {
    Write-Host "当前未安装 TextIn xParse 测试 Connector，无需恢复。"
    exit 0
}
if (-not (Test-Path -LiteralPath $CatalogBackup -PathType Leaf)) {
    throw "未找到 WorkBuddy Connector 注册表备份：${CatalogBackup}。为避免覆盖现有状态，已停止恢复。"
}

foreach ($Target in @(
    $ConnectorTestBackup,
    $CatalogTestBackup,
    $ProfileTestBackup,
    $CLITestBackup
)) {
    if (Test-Path -LiteralPath $Target) {
        throw "本次恢复的归档目标已存在：${Target}。请稍后重试。"
    }
}

if (-not (Test-Path -LiteralPath $TestBackupRoot -PathType Container)) {
    New-Item -ItemType Directory -Path $TestBackupRoot -Force | Out-Null
}
Move-Item -LiteralPath $ConnectorDir -Destination $ConnectorTestBackup
Move-Item -LiteralPath $CatalogFile -Destination $CatalogTestBackup
Move-Item -LiteralPath $CatalogBackup -Destination $CatalogFile
Get-Content -LiteralPath $CatalogFile -Raw | ConvertFrom-Json | Out-Null

if (Test-Path -LiteralPath $ConnectorBackup -PathType Container) {
    Move-Item -LiteralPath $ConnectorBackup -Destination $ConnectorDir
}
if (Test-Path -LiteralPath $ProfileDir -PathType Container) {
    Move-Item -LiteralPath $ProfileDir -Destination $ProfileTestBackup
}
if (Test-Path -LiteralPath $ProfileBackup -PathType Container) {
    Move-Item -LiteralPath $ProfileBackup -Destination $ProfileDir
}
if (Test-Path -LiteralPath $CLIPath -PathType Leaf) {
    Move-Item -LiteralPath $CLIPath -Destination $CLITestBackup
}
if (Test-Path -LiteralPath $CLIBackup -PathType Leaf) {
    Move-Item -LiteralPath $CLIBackup -Destination $CLIPath
}

Write-Host "已恢复执行测试脚本前的 WorkBuddy marketplace、Connector、CLI 和 profile 状态。"
Write-Host "本次测试 Connector 已归档到：${ConnectorTestBackup}"
Write-Host ""
Write-Host "请完全退出并重新打开 WorkBuddy，使恢复后的状态生效。"
