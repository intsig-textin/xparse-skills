$ErrorActionPreference = "Stop"

$Version = if ($env:XPARSER_VERSION) { $env:XPARSER_VERSION } else { "v2.1.0-workbuddy-test.1" }
$DownloadBase = if ($env:XPARSER_DOWNLOAD_BASE) {
    $env:XPARSER_DOWNLOAD_BASE
} else {
    "https://dllf.intsig.net/download/2026/Solution/xparse-cli"
}
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
$DownloadFile = "${ConnectorFile}.download"

if (-not (Test-Path -LiteralPath $ConnectorFile -PathType Leaf)) {
    throw "未找到 TextIn xParse 的 WorkBuddy 配置：${ConnectorFile}。请先安装并启动一次 WorkBuddy。"
}

try {
    if ($env:XPARSE_TEST_CONNECTOR_FILE) {
        Copy-Item -LiteralPath $env:XPARSE_TEST_CONNECTOR_FILE -Destination $DownloadFile -Force
    } else {
        $ConnectorURL = if ($env:XPARSE_TEST_CONNECTOR_URL) {
            $env:XPARSE_TEST_CONNECTOR_URL
        } else {
            "${DownloadBase}/${Version}/workbuddy-cli.json"
        }
        Write-Host "正在下载 WorkBuddy 测试配置：${ConnectorURL}"
        Invoke-WebRequest -Uri $ConnectorURL -OutFile $DownloadFile -UseBasicParsing
    }

    $TestConfig = Get-Content -LiteralPath $DownloadFile -Raw | ConvertFrom-Json
    if ($TestConfig.env.XPARSE_OAUTH_CLIENT_ID -ne "cli_textin_xparse_workbuddy") {
        throw "下载的文件不是预期的 WorkBuddy 测试配置。"
    }
    if ($TestConfig.authUrlDomain -ne "textin-sandbox.intsig.com") {
        throw "下载的配置没有指向 TextIn 测试环境。"
    }
    if ((Get-Content -LiteralPath $DownloadFile -Raw).Contains("/latest/")) {
        throw "测试配置不能引用 latest 目录。"
    }

    $CurrentConfig = Get-Content -LiteralPath $ConnectorFile -Raw | ConvertFrom-Json
    if ($CurrentConfig.env.XPARSE_OAUTH_CLIENT_ID -eq "cli_textin_xparse_workbuddy") {
        Move-Item -LiteralPath $DownloadFile -Destination $ConnectorFile -Force
        Write-Host "WorkBuddy 已在使用 TextIn 测试配置，配置已刷新。"
    } else {
        if (Test-Path -LiteralPath $ConnectorBackup) {
            throw "检测到未恢复的生产配置备份：${ConnectorBackup}。请先运行恢复脚本。"
        }
        if ((Test-Path -LiteralPath $ProfileDir -PathType Container) -and
            (Test-Path -LiteralPath $ProfileBackup)) {
            throw "检测到未恢复的 WorkBuddy profile 备份：${ProfileBackup}。请先运行恢复脚本。"
        }

        Move-Item -LiteralPath $ConnectorFile -Destination $ConnectorBackup
        if (Test-Path -LiteralPath $ProfileDir -PathType Container) {
            Move-Item -LiteralPath $ProfileDir -Destination $ProfileBackup
        }
        Move-Item -LiteralPath $DownloadFile -Destination $ConnectorFile
        Write-Host "已备份生产配置，并切换到 TextIn 测试环境。"
    }
} finally {
    if (Test-Path -LiteralPath $DownloadFile -PathType Leaf) {
        Remove-Item -LiteralPath $DownloadFile -Force
    }
}

Write-Host ""
Write-Host "请完全退出并重新打开 WorkBuddy，然后在 TextIn xParse 中点击“连接”。"
Write-Host "WorkBuddy 将自动安装指定测试版 CLI，并打开 TextIn 测试环境授权页。"
Write-Host "测试结束后请运行 restore-workbuddy-production.ps1 恢复生产配置和原登录态。"
