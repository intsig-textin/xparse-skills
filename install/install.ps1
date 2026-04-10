# xParser CLI installer for Windows
# Usage: irm https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.ps1 | iex
#
# Environment variables:
#   XPARSER_VERSION   - version to install (default: "latest")
#   XPARSER_BASE_URL  - override download base URL
#   INSTALL_DIR       - install directory (default: $HOME\.xparse-cli\bin)

$ErrorActionPreference = "Stop"

# Ensure TLS 1.2+ for older PowerShell versions
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
} catch {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
}

# ── helpers ──

function Write-Info  { param([string]$Msg) Write-Host "  $Msg" }
function Write-Ok    { param([string]$Msg) Write-Host "  + $Msg" }
function Write-Err   { param([string]$Msg) Write-Host "  x $Msg" -ForegroundColor Red }

# ── config ──

$Version    = if ($env:XPARSER_VERSION)  { $env:XPARSER_VERSION }  else { "latest" }
$BaseURL    = if ($env:XPARSER_BASE_URL) { $env:XPARSER_BASE_URL } else { "https://dllf.intsig.net/download/2026/Solution/xparse-cli" }
$InstallDir = if ($env:INSTALL_DIR)      { $env:INSTALL_DIR }      else { "$HOME\.xparse-cli\bin" }

# ── detect platform ──

$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Err "32-bit Windows is not supported"; exit 1
}

$Binary   = "xparse-cli-windows-${Arch}.exe"
$URL      = "${BaseURL}/${Version}/${Binary}"
$DestPath = Join-Path $InstallDir "xparse-cli.exe"

# ── check if upgrading ──

if (Test-Path $DestPath) {
    try {
        $OldVer = & $DestPath version 2>&1 | Select-Object -First 1
        Write-Info "Upgrading from ${OldVer}..."
    } catch {
        Write-Info "Upgrading existing installation..."
    }
}

# ── download ──

Write-Info "Downloading xparse-cli ${Version} for windows/${Arch}..."
Write-Info "${URL}"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

try {
    Invoke-WebRequest -Uri $URL -OutFile $DestPath -UseBasicParsing

    if (Get-Command Unblock-File -ErrorAction SilentlyContinue) {
        Unblock-File -Path $DestPath -ErrorAction SilentlyContinue
    }
} catch {
    Write-Err "Download failed: $_"
    exit 1
}

if (-not (Test-Path $DestPath) -or (Get-Item $DestPath).Length -eq 0) {
    Write-Err "Download failed or file is empty"
    exit 1
}

# ── configure PATH ──

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = if ([string]::IsNullOrWhiteSpace($UserPath)) { $InstallDir } else { "$UserPath;$InstallDir" }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Ok "Added $InstallDir to user PATH"
}

# ── verify ──

Write-Host ""
Write-Host "  =========================================================="
Write-Host "  " -NoNewline; & $DestPath version
Write-Host "  =========================================================="
Write-Host ""
Write-Ok "Installed: $DestPath"
