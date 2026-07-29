$ErrorActionPreference = "Stop"

$Version = if ($env:XPARSER_VERSION) { $env:XPARSER_VERSION } else { "v2.1.0-1" }
$DownloadBase = if ($env:XPARSER_DOWNLOAD_BASE) {
    $env:XPARSER_DOWNLOAD_BASE
} else {
    "https://dllf.intsig.net/download/2026/Solution/xparse-cli"
}
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
$MarketplaceIconsDir = if ($env:WORKBUDDY_MARKETPLACE_ICONS_DIR) {
    $env:WORKBUDDY_MARKETPLACE_ICONS_DIR
} else {
    Join-Path $MarketplaceRoot "icons"
}
$MarketplaceIcon = Join-Path $MarketplaceIconsDir "textin-xparse.png"
$ActiveSkillsDir = if ($env:WORKBUDDY_CONNECTOR_SKILLS_DIR) {
    $env:WORKBUDDY_CONNECTOR_SKILLS_DIR
} else {
    Join-Path $UserHome ".workbuddy\connectors\skills\connector-textin-xparse"
}
$CLIPath = if ($env:XPARSE_CLI_PATH) {
    $env:XPARSE_CLI_PATH
} else {
    Join-Path $UserHome ".xparse-cli\bin\xparse-cli.exe"
}
$StageDir = Join-Path $ConnectorsDir ".textin-xparse.prod-download.$PID"
$CatalogDownload = "${CatalogFile}.textin-xparse-prod.download.${PID}"
$EntryDownload = "${CatalogFile}.textin-xparse-entry.download.${PID}"
$OldConnector = "${ConnectorDir}.prod-install-old.${PID}"
$OldMarketplaceIcon = "${MarketplaceIcon}.prod-install-old.${PID}"
$OldActiveSkills = "${ActiveSkillsDir}.prod-install-old.${PID}"
$OldCatalog = "${CatalogFile}.textin-xparse-prod-install-old.${PID}"
$ExpectedIconSHA256 = "2f5159dd77b1d4625d44ab5ac30a4b40417be1280083ae4c86ea674d485c4234"
$UTF8NoBOM = New-Object System.Text.UTF8Encoding($false)
$UTF8Strict = New-Object System.Text.UTF8Encoding($false, $true)

function Read-Utf8Text {
    param([string]$Path)
    return [System.IO.File]::ReadAllText($Path, $UTF8Strict)
}

function Read-Utf8Json {
    param([string]$Path)
    return (Read-Utf8Text $Path | ConvertFrom-Json)
}

function Get-ProdAsset {
    param(
        [string]$RemoteName,
        [string]$Destination
    )
    if ($env:XPARSE_PROD_ASSET_DIR) {
        Copy-Item -LiteralPath (Join-Path $env:XPARSE_PROD_ASSET_DIR $RemoteName) `
            -Destination $Destination
    } else {
        $AssetURL = "${DownloadBase}/${Version}/${RemoteName}"
        Write-Host "正在下载：${AssetURL}"
        Invoke-WebRequest -Uri $AssetURL -OutFile $Destination -UseBasicParsing
    }
}

function Remove-PathIfPresent {
    param([string]$Path)
    if (Test-Path -LiteralPath $Path -PathType Container) {
        Remove-Item -LiteralPath $Path -Recurse -Force
    } elseif (Test-Path -LiteralPath $Path -PathType Leaf) {
        Remove-Item -LiteralPath $Path -Force
    }
}

if (-not (Test-Path -LiteralPath $CatalogFile -PathType Leaf)) {
    throw "未找到 WorkBuddy Connector 注册表：${CatalogFile}。请先安装并启动一次 WorkBuddy。"
}

$UnresolvedTestState = @(
    (Join-Path $ConnectorDir ".workbuddy-test"),
    "${ConnectorDir}.production.bak",
    "${MarketplaceIcon}.production.bak",
    "${CatalogFile}.textin-xparse.production.bak",
    (Join-Path $UserHome ".xparse-cli\profiles\workbuddy.production.bak"),
    (Join-Path $UserHome ".xparse-cli\bin\xparse-cli.exe.production.bak"),
    "${ActiveSkillsDir}.production.bak"
) | Where-Object { Test-Path -LiteralPath $_ }
if ($UnresolvedTestState.Count -gt 0) {
    $StateList = ($UnresolvedTestState | ForEach-Object { "  - $_" }) -join "`n"
    throw "检测到尚未恢复的 WorkBuddy 测试状态：`n${StateList}`n请先运行对应测试包的恢复脚本。"
}

foreach ($Temporary in @(
    $StageDir,
    $CatalogDownload,
    $EntryDownload,
    $OldConnector,
    $OldMarketplaceIcon,
    $OldActiveSkills,
    $OldCatalog
)) {
    if (Test-Path -LiteralPath $Temporary) {
        throw "安装暂存路径已存在：${Temporary}。请稍后重试。"
    }
}

$ConnectorInstalled = $false
$IconInstalled = $false
$ActiveSkillsInstalled = $false
$CatalogInstalled = $false
$ProfileConfigured = $false
$Succeeded = $false

try {
    if (-not (Test-Path -LiteralPath $ConnectorsDir -PathType Container)) {
        New-Item -ItemType Directory -Path $ConnectorsDir -Force | Out-Null
    }
    New-Item -ItemType Directory -Path $StageDir | Out-Null

    Get-ProdAsset "workbuddy-cli.json" (Join-Path $StageDir "cli.json")
    Get-ProdAsset "workbuddy-connector-meta.json" (Join-Path $StageDir "connector-meta.json")
    Get-ProdAsset "workbuddy-icon.png" (Join-Path $StageDir "icon.png")
    Get-ProdAsset "workbuddy-marketplace-entry.json" $EntryDownload
    Get-ProdAsset "workbuddy-xparse-parse.zip" (Join-Path $StageDir "xparse-parse.zip")

    $StageSkillsDir = Join-Path $StageDir "skills"
    New-Item -ItemType Directory -Path $StageSkillsDir | Out-Null
    Expand-Archive -LiteralPath (Join-Path $StageDir "xparse-parse.zip") `
        -DestinationPath $StageSkillsDir
    Remove-Item -LiteralPath (Join-Path $StageDir "xparse-parse.zip") -Force

    $ProdCLIText = Read-Utf8Text (Join-Path $StageDir "cli.json")
    $ProdCLI = $ProdCLIText | ConvertFrom-Json
    $ProdMeta = Read-Utf8Json (Join-Path $StageDir "connector-meta.json")
    $ProdEntry = Read-Utf8Json $EntryDownload
    if ($ProdCLI.env.XPARSE_OAUTH_CLIENT_ID -ne "cli_textin_xparse_workbuddy") {
        throw "下载的 CLI 配置没有使用 WorkBuddy OAuth client。"
    }
    if ($ProdCLI.authUrlDomain -ne "api.textin.com") {
        throw "下载的 CLI 配置没有指向 TextIn 正式环境。"
    }
    if ($ProdCLI.env.XPARSE_BASE_URL -ne "https://api.textin.com") {
        throw "下载的 CLI 配置没有强制使用 TextIn 正式 API。"
    }
    if (-not $ProdCLI.init.darwin.Contains("/${Version}/install.sh")) {
        throw "下载的 CLI 配置没有固定到 ${Version}。"
    }
    if (
        $ProdCLIText.Contains("textin-api-pre.intsig.com") -or
        $ProdCLIText.Contains("/latest/")
    ) {
        throw "正式 Connector 配置包含 pre 或 latest 引用。"
    }
    if ($ProdMeta.source -ne "textin-xparse") {
        throw "下载的 Connector 元数据无效。"
    }
    if ($ProdEntry.id -ne "textin-xparse") {
        throw "下载的 marketplace 注册项无效。"
    }
    $IconSHA256 = (Get-FileHash -LiteralPath (Join-Path $StageDir "icon.png") `
        -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($IconSHA256 -ne $ExpectedIconSHA256) {
        throw "下载的 Connector 图标不是预期的 TextIn xParse Logo。"
    }
    if (-not (Test-Path -LiteralPath `
        (Join-Path $StageSkillsDir "xparse-parse\SKILL.md") -PathType Leaf)) {
        throw "下载的 Connector Skill 不完整：缺少 xparse-parse/SKILL.md。"
    }
    if (Test-Path -LiteralPath (Join-Path $StageSkillsDir "xparse-doc-tools")) {
        throw "正式 Connector 只能包含 xparse-parse Skill。"
    }

    $Catalog = Read-Utf8Json $CatalogFile
    $RemainingEntries = @(
        $Catalog.connectors | Where-Object { $_.id -ne $ProdEntry.id }
    )
    $Catalog.connectors = @($ProdEntry) + $RemainingEntries
    $CatalogJSON = $Catalog | ConvertTo-Json -Depth 100
    [System.IO.File]::WriteAllText($CatalogDownload, "${CatalogJSON}`n", $UTF8NoBOM)
    Read-Utf8Json $CatalogDownload | Out-Null

    Copy-Item -LiteralPath $CatalogFile -Destination $OldCatalog
    if (Test-Path -LiteralPath $ConnectorDir -PathType Container) {
        Move-Item -LiteralPath $ConnectorDir -Destination $OldConnector
    }
    Move-Item -LiteralPath $StageDir -Destination $ConnectorDir
    $ConnectorInstalled = $true

    if (-not (Test-Path -LiteralPath $MarketplaceIconsDir -PathType Container)) {
        New-Item -ItemType Directory -Path $MarketplaceIconsDir -Force | Out-Null
    }
    if (Test-Path -LiteralPath $MarketplaceIcon -PathType Leaf) {
        Move-Item -LiteralPath $MarketplaceIcon -Destination $OldMarketplaceIcon
    }
    Copy-Item -LiteralPath (Join-Path $ConnectorDir "icon.png") `
        -Destination $MarketplaceIcon
    $IconInstalled = $true

    if (Test-Path -LiteralPath $ActiveSkillsDir -PathType Container) {
        Move-Item -LiteralPath $ActiveSkillsDir -Destination $OldActiveSkills
    }
    $ActiveSkillsParent = Split-Path -Parent $ActiveSkillsDir
    if (-not (Test-Path -LiteralPath $ActiveSkillsParent -PathType Container)) {
        New-Item -ItemType Directory -Path $ActiveSkillsParent -Force | Out-Null
    }
    Copy-Item -LiteralPath (Join-Path $ConnectorDir "skills") `
        -Destination $ActiveSkillsDir -Recurse
    $ActiveSkillsInstalled = $true

    Move-Item -LiteralPath $CatalogDownload -Destination $CatalogFile -Force
    $CatalogInstalled = $true

    if (Test-Path -LiteralPath $CLIPath -PathType Leaf) {
        & $CLIPath --profile workbuddy config set base_url https://api.textin.com
        if ($LASTEXITCODE -ne 0) {
            throw "无法把现有 WorkBuddy CLI profile 切换到 TextIn 正式环境。"
        }
        $ProfileConfigured = $true
    }
    $Succeeded = $true
} catch {
    if ($CatalogInstalled -and (Test-Path -LiteralPath $OldCatalog -PathType Leaf)) {
        Copy-Item -LiteralPath $OldCatalog -Destination $CatalogFile -Force
    }
    if ($ActiveSkillsInstalled) {
        Remove-PathIfPresent $ActiveSkillsDir
        if (Test-Path -LiteralPath $OldActiveSkills -PathType Container) {
            Move-Item -LiteralPath $OldActiveSkills -Destination $ActiveSkillsDir
        }
    }
    if ($IconInstalled) {
        Remove-PathIfPresent $MarketplaceIcon
        if (Test-Path -LiteralPath $OldMarketplaceIcon -PathType Leaf) {
            Move-Item -LiteralPath $OldMarketplaceIcon -Destination $MarketplaceIcon
        }
    }
    if ($ConnectorInstalled) {
        Remove-PathIfPresent $ConnectorDir
        if (Test-Path -LiteralPath $OldConnector -PathType Container) {
            Move-Item -LiteralPath $OldConnector -Destination $ConnectorDir
        }
    }
    throw
} finally {
    Remove-PathIfPresent $StageDir
    Remove-PathIfPresent $CatalogDownload
    Remove-PathIfPresent $EntryDownload
    Remove-PathIfPresent $OldCatalog
    if ($Succeeded) {
        Remove-PathIfPresent $OldConnector
        Remove-PathIfPresent $OldMarketplaceIcon
        Remove-PathIfPresent $OldActiveSkills
    }
}

Write-Host "已安装 TextIn xParse 正式 Connector（${Version}）。"
if ($ProfileConfigured) {
    Write-Host "已将现有 WorkBuddy CLI profile 切换到 TextIn 正式环境，登录凭证保持不变。"
} else {
    Write-Host "尚未检测到 CLI；WorkBuddy 首次连接安装 CLI 后会自动写入正式环境配置。"
}
Write-Host ""
Write-Host "请完全退出并重新打开 WorkBuddy，然后在“TextIn xParse·智能文档解析”中点击“连接”。"
