$ErrorActionPreference = "Stop"

$Version = if ($env:XPARSER_VERSION) { $env:XPARSER_VERSION } else { "v2.2.0-workbuddy-test.3" }
$NpmVersion = if ($env:XPARSE_NPM_VERSION) { $env:XPARSE_NPM_VERSION } else { "2.2.1-beta.1" }
$ExpectedAuthDomain = if ($env:XPARSE_EXPECTED_AUTH_DOMAIN) {
    $env:XPARSE_EXPECTED_AUTH_DOMAIN
} else {
    "textin-sandbox.intsig.com"
}
$ProfileBaseURL = if ($env:XPARSE_PROFILE_BASE_URL) {
    $env:XPARSE_PROFILE_BASE_URL
} else {
    "https://textin-sandbox.intsig.com"
}
$EnvironmentLabel = if ($env:XPARSE_ENVIRONMENT_LABEL) {
    $env:XPARSE_ENVIRONMENT_LABEL
} else {
    "test"
}
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
$CLIPath = if ($env:XPARSE_CLI_PATH) {
    $env:XPARSE_CLI_PATH
} else {
    Join-Path $NpmPrefix "xparse-cli.cmd"
}
$ActiveSkillsDir = if ($env:WORKBUDDY_CONNECTOR_SKILLS_DIR) {
    $env:WORKBUDDY_CONNECTOR_SKILLS_DIR
} else {
    Join-Path $UserHome ".workbuddy\connectors\skills\connector-textin-xparse"
}
$ActiveSkillsBackup = "${ActiveSkillsDir}.production.bak"
$StageDir = Join-Path $ConnectorsDir ".textin-xparse.test-download.$PID"
$CatalogDownload = "${CatalogFile}.textin-xparse-test.download.${PID}"
$EntryDownload = "${CatalogFile}.textin-xparse-entry.download.${PID}"
$MarkerFile = Join-Path $ConnectorDir ".workbuddy-test"
$UTF8NoBOM = New-Object System.Text.UTF8Encoding($false)
$UTF8Strict = New-Object System.Text.UTF8Encoding($false, $true)
$ExpectedIconSHA256 = "2f5159dd77b1d4625d44ab5ac30a4b40417be1280083ae4c86ea674d485c4234"

function Read-Utf8Text {
    param([string]$Path)
    return [System.IO.File]::ReadAllText($Path, $UTF8Strict)
}

function Read-Utf8Json {
    param([string]$Path)
    return (Read-Utf8Text $Path | ConvertFrom-Json)
}

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
        (Join-Path $StageDir "xparse-parse.zip"),
        (Join-Path $StageDir ".workbuddy-test"),
        $CatalogDownload,
        $EntryDownload
    )) {
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            Remove-Item -LiteralPath $Path -Force
        }
    }
    if (Test-Path -LiteralPath $StageDir -PathType Container) {
        Remove-Item -LiteralPath $StageDir -Recurse -Force
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
    Get-TestAsset "workbuddy-xparse-parse.zip" (Join-Path $StageDir "xparse-parse.zip")

    $StageSkillsDir = Join-Path $StageDir "skills"
    New-Item -ItemType Directory -Path $StageSkillsDir | Out-Null
    Expand-Archive -LiteralPath (Join-Path $StageDir "xparse-parse.zip") `
        -DestinationPath $StageSkillsDir
    Remove-Item -LiteralPath (Join-Path $StageDir "xparse-parse.zip") -Force

    $TestCLI = Read-Utf8Json (Join-Path $StageDir "cli.json")
    $TestMeta = Read-Utf8Json (Join-Path $StageDir "connector-meta.json")
    $TestEntry = Read-Utf8Json $EntryDownload
    if ($TestCLI.env.XPARSE_OAUTH_CLIENT_ID -ne "cli_textin_xparse_workbuddy") {
        throw "下载的 CLI 配置不是预期的 WorkBuddy test 配置。"
    }
    if ($TestCLI.authUrlDomain -ne $ExpectedAuthDomain) {
        throw "下载的 CLI 配置没有指向 TextIn ${EnvironmentLabel} 环境。"
    }
    if ($TestMeta.source -ne "textin-xparse") {
        throw "下载的 Connector 元数据无效。"
    }
    if ($TestEntry.id -ne "textin-xparse") {
        throw "下载的 marketplace 注册项无效。"
    }
    if ((Read-Utf8Text (Join-Path $StageDir "cli.json")).Contains("/latest/")) {
        throw "test 配置不能引用 latest 目录。"
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
        throw "test Connector 只能包含 xparse-parse Skill。"
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $StageDir ".workbuddy-test"),
        "${Version}`n",
        $UTF8NoBOM
    )

    if (Test-Path -LiteralPath $MarkerFile -PathType Leaf) {
        $OldSkills = "${ConnectorDir}\skills.test-refresh.${PID}"
        $ActiveRefresh = "${ActiveSkillsDir}.test-refresh.${PID}"
        if (Test-Path -LiteralPath $OldSkills) {
            throw "Connector Skill 刷新暂存目录已存在：${OldSkills}。"
        }
        if (Test-Path -LiteralPath $ActiveRefresh) {
            throw "已激活 Skill 刷新暂存目录已存在：${ActiveRefresh}。"
        }
        if (
            -not (Test-Path -LiteralPath $ActiveSkillsBackup) -and
            (Test-Path -LiteralPath $ActiveSkillsDir -PathType Container)
        ) {
            # Migration from test packages that predate activated-Skill
            # backup support. Preserve the only existing copy for restore.
            Move-Item -LiteralPath $ActiveSkillsDir -Destination $ActiveSkillsBackup
        }
        if (
            -not (Test-Path -LiteralPath $MarketplaceIconBackup) -and
            (Test-Path -LiteralPath $MarketplaceIcon -PathType Leaf)
        ) {
            Move-Item -LiteralPath $MarketplaceIcon -Destination $MarketplaceIconBackup
        }
        foreach ($Name in @("cli.json", "connector-meta.json", "icon.png", ".workbuddy-test")) {
            Move-Item -LiteralPath (Join-Path $StageDir $Name) `
                -Destination (Join-Path $ConnectorDir $Name) -Force
        }
        if (-not (Test-Path -LiteralPath $MarketplaceIconsDir -PathType Container)) {
            New-Item -ItemType Directory -Path $MarketplaceIconsDir -Force | Out-Null
        }
        Copy-Item -LiteralPath (Join-Path $ConnectorDir "icon.png") `
            -Destination $MarketplaceIcon -Force
        $ConnectorSkills = Join-Path $ConnectorDir "skills"
        if (Test-Path -LiteralPath $ConnectorSkills -PathType Container) {
            Move-Item -LiteralPath $ConnectorSkills -Destination $OldSkills
        }
        Move-Item -LiteralPath $StageSkillsDir -Destination $ConnectorSkills
        if (Test-Path -LiteralPath $OldSkills -PathType Container) {
            Remove-Item -LiteralPath $OldSkills -Recurse -Force
        }
        if (Test-Path -LiteralPath $ActiveSkillsDir -PathType Container) {
            Move-Item -LiteralPath $ActiveSkillsDir -Destination $ActiveRefresh
        }
        $ActiveSkillsParent = Split-Path -Parent $ActiveSkillsDir
        if (-not (Test-Path -LiteralPath $ActiveSkillsParent -PathType Container)) {
            New-Item -ItemType Directory -Path $ActiveSkillsParent -Force | Out-Null
        }
        Copy-Item -LiteralPath $ConnectorSkills -Destination $ActiveSkillsDir -Recurse
        if (Test-Path -LiteralPath $ActiveRefresh -PathType Container) {
            Remove-Item -LiteralPath $ActiveRefresh -Recurse -Force
        }
        Write-Host "WorkBuddy 已安装 TextIn xParse test Connector，文件已刷新。"
    } else {
        foreach ($Backup in @(
            $CatalogBackup,
            $ConnectorBackup,
            $MarketplaceIconBackup,
            $ProfileBackup,
            $NpmBackup,
            $ActiveSkillsBackup
        )) {
            if (Test-Path -LiteralPath $Backup) {
                throw "检测到未恢复的备份：${Backup}。请先运行恢复脚本。"
            }
        }

        $Catalog = Read-Utf8Json $CatalogFile
        $ExistingEntries = @(
            $Catalog.connectors | Where-Object { $_.id -eq "textin-xparse" }
        )
        if ($ExistingEntries.Count -eq 0) {
            $Catalog.connectors = @($TestEntry) + @($Catalog.connectors)
        }
        $CatalogJSON = $Catalog | ConvertTo-Json -Depth 100
        [System.IO.File]::WriteAllText($CatalogDownload, "${CatalogJSON}`n", $UTF8NoBOM)
        Read-Utf8Json $CatalogDownload | Out-Null

        Copy-Item -LiteralPath $CatalogFile -Destination $CatalogBackup
        if (Test-Path -LiteralPath $ConnectorDir -PathType Container) {
            Move-Item -LiteralPath $ConnectorDir -Destination $ConnectorBackup
        }
        if (Test-Path -LiteralPath $MarketplaceIcon -PathType Leaf) {
            Move-Item -LiteralPath $MarketplaceIcon -Destination $MarketplaceIconBackup
        }
        if (Test-Path -LiteralPath $ProfileDir -PathType Container) {
            Move-Item -LiteralPath $ProfileDir -Destination $ProfileBackup
        }
        if (Test-Path -LiteralPath $NpmPrefix -PathType Container) {
            Move-Item -LiteralPath $NpmPrefix -Destination $NpmBackup
        }
        if (Test-Path -LiteralPath $ActiveSkillsDir -PathType Container) {
            Move-Item -LiteralPath $ActiveSkillsDir -Destination $ActiveSkillsBackup
        }
        Move-Item -LiteralPath $CatalogDownload -Destination $CatalogFile -Force
        Move-Item -LiteralPath $StageDir -Destination $ConnectorDir
        if (-not (Test-Path -LiteralPath $MarketplaceIconsDir -PathType Container)) {
            New-Item -ItemType Directory -Path $MarketplaceIconsDir -Force | Out-Null
        }
        Copy-Item -LiteralPath (Join-Path $ConnectorDir "icon.png") `
            -Destination $MarketplaceIcon -Force
        $ActiveSkillsParent = Split-Path -Parent $ActiveSkillsDir
        if (-not (Test-Path -LiteralPath $ActiveSkillsParent -PathType Container)) {
            New-Item -ItemType Directory -Path $ActiveSkillsParent -Force | Out-Null
        }
        Copy-Item -LiteralPath (Join-Path $ConnectorDir "skills") `
            -Destination $ActiveSkillsDir -Recurse
        Write-Host "已注入 TextIn xParse test Connector，并备份执行前的 WorkBuddy 状态。"
    }
} finally {
    Clear-TestDownloads
}

if (-not $env:XPARSE_TEST_ASSET_DIR) {
    if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
        throw "缺少 Node.js 18 或更高版本，无法安装 npm CLI。"
    }
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
        throw "缺少 npm，无法安装 xparse-cli。"
    }
    $NodeMajor = [int](& node -p "process.versions.node.split('.')[0]")
    if ($NodeMajor -lt 18) {
        throw "Node.js 版本过低（${NodeMajor}），xparse-cli 需要 Node.js 18 或更高版本。"
    }
    Write-Host "正在通过 npm 安装 test CLI：xparse-cli@${NpmVersion}"
    & npm install --global --prefix $NpmPrefix "xparse-cli@${NpmVersion}"
    if ($LASTEXITCODE -ne 0) {
        throw "npm 安装 xparse-cli@${NpmVersion} 失败。请执行恢复脚本。"
    }
    if (-not (Test-Path -LiteralPath $CLIPath -PathType Leaf)) {
        throw "test CLI 安装失败：未找到 ${CLIPath}。请执行恢复脚本。"
    }
    & $CLIPath --profile workbuddy config set base_url $ProfileBaseURL
    if ($LASTEXITCODE -ne 0) {
        throw "CLI 无法写入 ${EnvironmentLabel} Profile。请执行恢复脚本。"
    }
}

Write-Host ""
Write-Host "一键安装已完成：Connector、xparse-parse Skill、npm CLI ${NpmVersion} 和 ${EnvironmentLabel} Profile 均已就绪。"
Write-Host "请完全退出并重新打开 WorkBuddy；需要验收登录归因时，再在 TextIn xParse 中点击“连接”。"
Write-Host "测试结束后请执行对应版本目录中的 restore-workbuddy-production.ps1 恢复执行前状态。"
