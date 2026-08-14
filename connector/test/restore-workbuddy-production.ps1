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
$MarketplaceIconsDir = if ($env:WORKBUDDY_MARKETPLACE_ICONS_DIR) {
    $env:WORKBUDDY_MARKETPLACE_ICONS_DIR
} else {
    Join-Path $MarketplaceRoot "icons"
}
$MarketplaceIcon = Join-Path $MarketplaceIconsDir "textin-xparse.png"
$MarketplaceIconBackup = "${MarketplaceIcon}.production.bak"
$CatalogBackup = "${CatalogFile}.textin-xparse.production.bak"
$ProfileDir = if ($env:XPARSE_WORKBUDDY_PROFILE_DIR) {
    $env:XPARSE_WORKBUDDY_PROFILE_DIR
} else {
    Join-Path $UserHome ".xparse-cli\profiles\workbuddy"
}
$ProfileBackup = "${ProfileDir}.production.bak"
$NpmPrefix = if ($env:XPARSE_NPM_PREFIX) {
    $env:XPARSE_NPM_PREFIX
} else {
    Join-Path $UserHome ".xparse-cli\npm"
}
$NpmBackup = "${NpmPrefix}.production.bak"
$ActiveSkillsDir = if ($env:WORKBUDDY_CONNECTOR_SKILLS_DIR) {
    $env:WORKBUDDY_CONNECTOR_SKILLS_DIR
} else {
    Join-Path $UserHome ".workbuddy\connectors\skills\connector-textin-xparse"
}
$ActiveSkillsBackup = "${ActiveSkillsDir}.production.bak"
$Timestamp = Get-Date -Format "yyyyMMddHHmmss"
$TestBackupRoot = if ($env:WORKBUDDY_TEST_BACKUP_ROOT) {
    $env:WORKBUDDY_TEST_BACKUP_ROOT
} else {
    Join-Path $MarketplaceRoot ".textin-xparse-test-backups"
}
$ConnectorTestBackup = Join-Path $TestBackupRoot "connector.${Timestamp}"
$CatalogTestBackup = Join-Path $TestBackupRoot "connectors.${Timestamp}.json"
$ProfileTestBackup = "${ProfileDir}.test.${Timestamp}.bak"
$NpmTestBackup = "${NpmPrefix}.test.${Timestamp}.bak"
$ActiveSkillsTestBackup = Join-Path $TestBackupRoot "activated-skills.${Timestamp}"
$MarketplaceIconTestBackup = Join-Path $TestBackupRoot "marketplace-icon.${Timestamp}.png"
$OrphanRecoveryRoot = Join-Path $TestBackupRoot "orphan-recovery.${Timestamp}"
$MarkerFile = Join-Path $ConnectorDir ".workbuddy-test"
$UTF8Strict = New-Object System.Text.UTF8Encoding($false, $true)

function Read-Utf8Json {
    param([string]$Path)
    return ([System.IO.File]::ReadAllText($Path, $UTF8Strict) | ConvertFrom-Json)
}

function Restore-OrphanedBackup {
    param(
        [string]$CurrentPath,
        [string]$BackupPath,
        [string]$ArchiveName
    )
    if (-not (Test-Path -LiteralPath $BackupPath)) {
        return
    }
    if (Test-Path -LiteralPath $CurrentPath) {
        Move-Item -LiteralPath $CurrentPath `
            -Destination (Join-Path $OrphanRecoveryRoot $ArchiveName)
    }
    $CurrentParent = Split-Path -Parent $CurrentPath
    if (-not (Test-Path -LiteralPath $CurrentParent -PathType Container)) {
        New-Item -ItemType Directory -Path $CurrentParent -Force | Out-Null
    }
    Move-Item -LiteralPath $BackupPath -Destination $CurrentPath
    Write-Host "已恢复孤立正式备份：${CurrentPath}"
}

if (-not (Test-Path -LiteralPath $MarkerFile -PathType Leaf)) {
    $OrphanedBackups = @(
        $CatalogBackup,
        $ConnectorBackup,
        $MarketplaceIconBackup,
        $ProfileBackup,
        $NpmBackup,
        $ActiveSkillsBackup
    ) | Where-Object { Test-Path -LiteralPath $_ }
    if ($OrphanedBackups.Count -gt 0) {
        $BackupList = ($OrphanedBackups | ForEach-Object { "  - $_" }) -join "`n"
        if (Test-Path -LiteralPath $OrphanRecoveryRoot) {
            throw "孤立备份恢复目录已存在：${OrphanRecoveryRoot}。请稍后重试。"
        }
        New-Item -ItemType Directory -Path $OrphanRecoveryRoot -Force | Out-Null
        Write-Host "未找到测试 Connector marker，开始自动恢复孤立正式备份："
        Write-Host $BackupList
        Restore-OrphanedBackup $CatalogFile $CatalogBackup "connectors.current.json"
        Restore-OrphanedBackup $ConnectorDir $ConnectorBackup "connector.current"
        Restore-OrphanedBackup $MarketplaceIcon $MarketplaceIconBackup `
            "marketplace-icon.current.png"
        Restore-OrphanedBackup $ProfileDir $ProfileBackup "profile.current"
        Restore-OrphanedBackup $NpmPrefix $NpmBackup "npm-cli.current"
        Restore-OrphanedBackup $ActiveSkillsDir $ActiveSkillsBackup `
            "activated-skills.current"
        if (Test-Path -LiteralPath $CatalogFile -PathType Leaf) {
            Read-Utf8Json $CatalogFile | Out-Null
        }
        Write-Host "已自动恢复孤立备份，原当前状态已归档到：${OrphanRecoveryRoot}"
        Write-Host "现在可以重新运行 enable 一键安装命令。"
        exit 0
    }
    Write-Host "当前未安装 TextIn xParse 测试 Connector，且没有遗留备份，无需恢复。"
    exit 0
}
if (-not (Test-Path -LiteralPath $CatalogBackup -PathType Leaf)) {
    throw "未找到 WorkBuddy Connector 注册表备份：${CatalogBackup}。为避免覆盖现有状态，已停止恢复。"
}

foreach ($Target in @(
    $ConnectorTestBackup,
    $CatalogTestBackup,
    $MarketplaceIconTestBackup,
    $ProfileTestBackup,
    $NpmTestBackup,
    $ActiveSkillsTestBackup
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
Read-Utf8Json $CatalogFile | Out-Null
if (Test-Path -LiteralPath $MarketplaceIcon -PathType Leaf) {
    Move-Item -LiteralPath $MarketplaceIcon -Destination $MarketplaceIconTestBackup
}
if (Test-Path -LiteralPath $MarketplaceIconBackup -PathType Leaf) {
    if (-not (Test-Path -LiteralPath $MarketplaceIconsDir -PathType Container)) {
        New-Item -ItemType Directory -Path $MarketplaceIconsDir -Force | Out-Null
    }
    Move-Item -LiteralPath $MarketplaceIconBackup -Destination $MarketplaceIcon
}

if (Test-Path -LiteralPath $ConnectorBackup -PathType Container) {
    Move-Item -LiteralPath $ConnectorBackup -Destination $ConnectorDir
}
if (Test-Path -LiteralPath $ProfileDir -PathType Container) {
    Move-Item -LiteralPath $ProfileDir -Destination $ProfileTestBackup
}
if (Test-Path -LiteralPath $ProfileBackup -PathType Container) {
    Move-Item -LiteralPath $ProfileBackup -Destination $ProfileDir
}
if (Test-Path -LiteralPath $NpmPrefix -PathType Container) {
    Move-Item -LiteralPath $NpmPrefix -Destination $NpmTestBackup
}
if (Test-Path -LiteralPath $NpmBackup -PathType Container) {
    Move-Item -LiteralPath $NpmBackup -Destination $NpmPrefix
}
if (Test-Path -LiteralPath $ActiveSkillsDir -PathType Container) {
    Move-Item -LiteralPath $ActiveSkillsDir -Destination $ActiveSkillsTestBackup
}
if (Test-Path -LiteralPath $ActiveSkillsBackup -PathType Container) {
    $ActiveSkillsParent = Split-Path -Parent $ActiveSkillsDir
    if (-not (Test-Path -LiteralPath $ActiveSkillsParent -PathType Container)) {
        New-Item -ItemType Directory -Path $ActiveSkillsParent -Force | Out-Null
    }
    Move-Item -LiteralPath $ActiveSkillsBackup -Destination $ActiveSkillsDir
}

Write-Host "已恢复执行测试脚本前的 WorkBuddy marketplace、Connector、npm CLI、profile 和已激活 Skill 状态。"
Write-Host "本次测试 Connector 已归档到：${ConnectorTestBackup}"
Write-Host ""
Write-Host "请完全退出并重新打开 WorkBuddy，使恢复后的状态生效。"
