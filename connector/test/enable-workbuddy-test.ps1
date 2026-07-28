$ErrorActionPreference = "Stop"

$Version = if ($env:XPARSER_VERSION) { $env:XPARSER_VERSION } else { "v2.1.0-workbuddy-test.3" }
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
$StageDir = Join-Path $ConnectorsDir ".textin-xparse.test-download.$PID"
$CatalogDownload = "${CatalogFile}.textin-xparse-test.download.${PID}"
$EntryDownload = "${CatalogFile}.textin-xparse-entry.download.${PID}"
$MarkerFile = Join-Path $ConnectorDir ".workbuddy-test"
$UTF8NoBOM = New-Object System.Text.UTF8Encoding($false)

function Get-TestAsset {
    param(
        [string]$RemoteName,
        [string]$Destination
    )
    if ($env:XPARSE_TEST_ASSET_DIR) {
        Copy-Item -LiteralPath (Join-Path $env:XPARSE_TEST_ASSET_DIR $RemoteName) `
            -Destination $Destination
    } else {
        $AssetURL = "${DownloadBase}/${Version}/${RemoteName}"
        Write-Host "正在下载：${AssetURL}"
        Invoke-WebRequest -Uri $AssetURL -OutFile $Destination -UseBasicParsing
    }
}

function Clear-TestDownloads {
    foreach ($Path in @(
        (Join-Path $StageDir "cli.json"),
        (Join-Path $StageDir "connector-meta.json"),
        (Join-Path $StageDir "icon.png"),
        (Join-Path $StageDir ".workbuddy-test"),
        $CatalogDownload,
        $EntryDownload
    )) {
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            Remove-Item -LiteralPath $Path -Force
        }
    }
    if (Test-Path -LiteralPath $StageDir -PathType Container) {
        Remove-Item -LiteralPath $StageDir
    }
}

if (-not (Test-Path -LiteralPath $CatalogFile -PathType Leaf)) {
    throw "未找到 WorkBuddy Connector 注册表：${CatalogFile}。请先安装并启动一次 WorkBuddy。"
}
if (-not (Test-Path -LiteralPath $ConnectorsDir -PathType Container)) {
    New-Item -ItemType Directory -Path $ConnectorsDir -Force | Out-Null
}
New-Item -ItemType Directory -Path $StageDir | Out-Null

try {
    Get-TestAsset "workbuddy-cli.json" (Join-Path $StageDir "cli.json")
    Get-TestAsset "workbuddy-connector-meta.json" (Join-Path $StageDir "connector-meta.json")
    Get-TestAsset "workbuddy-icon.png" (Join-Path $StageDir "icon.png")
    Get-TestAsset "workbuddy-marketplace-entry.json" $EntryDownload

    $TestCLI = Get-Content -LiteralPath (Join-Path $StageDir "cli.json") -Raw |
        ConvertFrom-Json
    $TestMeta = Get-Content -LiteralPath (Join-Path $StageDir "connector-meta.json") -Raw |
        ConvertFrom-Json
    $TestEntry = Get-Content -LiteralPath $EntryDownload -Raw | ConvertFrom-Json
    if ($TestCLI.env.XPARSE_OAUTH_CLIENT_ID -ne "cli_textin_xparse_workbuddy") {
        throw "下载的 CLI 配置不是预期的 WorkBuddy 测试配置。"
    }
    if ($TestCLI.authUrlDomain -ne "textin-sandbox.intsig.com") {
        throw "下载的 CLI 配置没有指向 TextIn 测试环境。"
    }
    if ($TestMeta.source -ne "textin-xparse") {
        throw "下载的 Connector 元数据无效。"
    }
    if ($TestEntry.id -ne "textin-xparse") {
        throw "下载的 marketplace 注册项无效。"
    }
    if ((Get-Content -LiteralPath (Join-Path $StageDir "cli.json") -Raw).Contains("/latest/")) {
        throw "测试配置不能引用 latest 目录。"
    }
    if ((Get-Item -LiteralPath (Join-Path $StageDir "icon.png")).Length -eq 0) {
        throw "下载的 Connector 图标为空。"
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $StageDir ".workbuddy-test"),
        "${Version}`n",
        $UTF8NoBOM
    )

    if (Test-Path -LiteralPath $MarkerFile -PathType Leaf) {
        foreach ($Name in @("cli.json", "connector-meta.json", "icon.png", ".workbuddy-test")) {
            Move-Item -LiteralPath (Join-Path $StageDir $Name) `
                -Destination (Join-Path $ConnectorDir $Name) -Force
        }
        Write-Host "WorkBuddy 已安装 TextIn xParse 测试 Connector，文件已刷新。"
    } else {
        foreach ($Backup in @(
            $CatalogBackup,
            $ConnectorBackup,
            $ProfileBackup,
            $CLIBackup
        )) {
            if (Test-Path -LiteralPath $Backup) {
                throw "检测到未恢复的备份：${Backup}。请先运行恢复脚本。"
            }
        }

        $Catalog = Get-Content -LiteralPath $CatalogFile -Raw | ConvertFrom-Json
        $ExistingEntries = @(
            $Catalog.connectors | Where-Object { $_.id -eq "textin-xparse" }
        )
        if ($ExistingEntries.Count -eq 0) {
            $Catalog.connectors = @($TestEntry) + @($Catalog.connectors)
        }
        $CatalogJSON = $Catalog | ConvertTo-Json -Depth 100
        [System.IO.File]::WriteAllText($CatalogDownload, "${CatalogJSON}`n", $UTF8NoBOM)
        Get-Content -LiteralPath $CatalogDownload -Raw | ConvertFrom-Json | Out-Null

        Copy-Item -LiteralPath $CatalogFile -Destination $CatalogBackup
        if (Test-Path -LiteralPath $ConnectorDir -PathType Container) {
            Move-Item -LiteralPath $ConnectorDir -Destination $ConnectorBackup
        }
        if (Test-Path -LiteralPath $ProfileDir -PathType Container) {
            Move-Item -LiteralPath $ProfileDir -Destination $ProfileBackup
        }
        if (Test-Path -LiteralPath $CLIPath -PathType Leaf) {
            Move-Item -LiteralPath $CLIPath -Destination $CLIBackup
        }
        Move-Item -LiteralPath $CatalogDownload -Destination $CatalogFile -Force
        Move-Item -LiteralPath $StageDir -Destination $ConnectorDir
        Write-Host "已注入 TextIn xParse 测试 Connector，并备份执行前的 WorkBuddy 状态。"
    }
} finally {
    Clear-TestDownloads
}

Write-Host ""
Write-Host "请完全退出并重新打开 WorkBuddy，然后在 TextIn xParse 中点击“连接”。"
Write-Host "WorkBuddy 将自动安装指定测试版 CLI，并打开 TextIn 测试环境授权页。"
Write-Host "测试结束后请运行 restore-workbuddy-production.ps1 恢复执行前状态。"
