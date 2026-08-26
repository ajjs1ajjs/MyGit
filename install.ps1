# MyGit (Go) - one-line installer/updater for Windows 10/11 & Server 2016+
# The SAME script installs on first run and safely UPDATES on subsequent runs:
#   - keeps config, database, repositories and users
#   - replaces only the binary and restarts the service
#
# Requirements: Git for Windows must be installed and on PATH (winget install Git.Git)
#
# Usage (PowerShell as Administrator):
#   irm https://raw.githubusercontent.com/ajjs1ajjs/MyGit/main/install.ps1 | iex
# or:
#   .\install.ps1 [-Version v3.0.13] [-Port 8060] [-SkipChecksum]
#
# Environment variables (same as install.sh): MYGIT_VERSION, MYGIT_PORT.

[CmdletBinding()]
param(
    [string]$Version = $env:MYGIT_VERSION,
    [int]$Port = $(if ($env:MYGIT_PORT) { [int]$env:MYGIT_PORT } else { 8060 }),
    [switch]$SkipChecksum
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo        = 'ajjs1ajjs/MyGit'
$ServiceName = 'mygit'
$InstallDir  = "$env:ProgramFiles\mygit"
$DataDir     = "$env:ProgramData\mygit"
$ReposDir    = "$DataDir\repos"
$Binary      = Join-Path $InstallDir 'mygit.exe'

if (-not $Version) { $Version = 'latest' }
if ($env:MYGIT_SKIP_CHECKSUM -eq '1') { $SkipChecksum = $true }

# --- Admin check --------------------------------------------------------------
$identity  = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "Please run as Administrator (PowerShell -> Run as Administrator)" -ForegroundColor Red
    exit 1
}

# --- Git dependency check -------------------------------------------------------
try {
    $gitVersion = (& git --version 2>$null)
} catch { $gitVersion = $null }
if (-not $gitVersion) {
    Write-Host "ERROR: git is not installed or not on PATH." -ForegroundColor Red
    Write-Host "MyGit shells out to the system git for smart HTTP. Install it with:"
    Write-Host "  winget install Git.Git"
    exit 1
}

# --- Find a free port starting at $Port (up to +30) ------------------------------
function Find-FreePort([int]$start) {
    for ($p = $start; $p -lt ($start + 30); $p++) {
        $listener = $null
        try {
            $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $p)
            $listener.Start(); $listener.Stop(); return $p
        } catch {
            if ($listener) { $listener.Stop() }
        }
    }
    return $start
}
$Port = Find-FreePort $Port

$isUpdate = (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) -or (Test-Path $Binary) -or (Test-Path "$DataDir\mygit.db")
$mode = if ($isUpdate) { 'Update' } else { 'Install' }

Write-Host "=============================================="
Write-Host "   MyGit - $mode"
Write-Host "=============================================="
Write-Host ""

$oldVersion = $null
if (Test-Path $Binary) {
    try { $oldVersion = (& $Binary --version 2>$null) } catch {}
}

# --- Architecture -----------------------------------------------------------------
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default {
        Write-Host "ERROR: unsupported architecture: $env:PROCESSOR_ARCHITECTURE" -ForegroundColor Red
        exit 1
    }
}
$binaryName = "mygit-windows-$arch.exe"

if ($Version -eq 'latest') {
    $downloadUrl  = "https://github.com/$Repo/releases/latest/download/$binaryName"
    $checksumsUrl = "https://github.com/$Repo/releases/latest/download/checksums.txt"
} else {
    $downloadUrl  = "https://github.com/$Repo/releases/download/$Version/$binaryName"
    $checksumsUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"
}

# --- Download ----------------------------------------------------------------------
Write-Host "[1/4] Downloading MyGit $Version ($binaryName)..."
$tmpBin = Join-Path $env:TEMP "$binaryName"
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpBin -UseBasicParsing
} catch {
    Write-Host "ERROR: download failed. Is release $Version published? ($_)" -ForegroundColor Red
    Remove-Item $tmpBin -ErrorAction SilentlyContinue
    exit 1
}

# --- Checksum (fail-closed) ----------------------------------------------------------
Write-Host "Verifying checksum..."
if ($SkipChecksum) {
    Write-Host "WARNING: checksum verification explicitly skipped. Installing unverified binary." -ForegroundColor Yellow
} else {
    $tmpSum = Join-Path $env:TEMP 'mygit-checksums.txt'
    try {
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $tmpSum -UseBasicParsing
    } catch {
        Write-Host "ERROR: could not download checksums.txt from $checksumsUrl." -ForegroundColor Red
        Write-Host "Refusing to install an unverified binary. Re-run once GitHub is reachable,"
        Write-Host "or use -SkipChecksum to explicitly bypass verification."
        Remove-Item $tmpBin, $tmpSum -ErrorAction SilentlyContinue
        exit 1
    }
    $expected = (Select-String -Path $tmpSum -Pattern ([regex]::Escape($binaryName)) |
        Select-Object -First 1).Line -split '\s+' | Select-Object -First 1
    if (-not $expected) {
        Write-Host "ERROR: checksums.txt has no entry for $binaryName; refusing to install." -ForegroundColor Red
        Remove-Item $tmpBin, $tmpSum -ErrorAction SilentlyContinue
        exit 1
    }
    $actual = (Get-FileHash -Algorithm SHA256 $tmpBin).Hash.ToLower()
    if ($expected.ToLower() -ne $actual) {
        Write-Host "ERROR: checksum mismatch for ${binaryName}. Expected $expected, got $actual." -ForegroundColor Red
        Remove-Item $tmpBin, $tmpSum -ErrorAction SilentlyContinue
        exit 1
    }
    Write-Host "Checksum OK."
    Remove-Item $tmpSum -ErrorAction SilentlyContinue
}

# --- Stop service before replacing binary --------------------------------------------
$svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($svc) {
    Write-Host "Stopping service $ServiceName..."
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    for ($i = 0; $i -lt 15 -and (Get-Process mygit -ErrorAction SilentlyContinue); $i++) {
        Start-Sleep -Seconds 1
    }
}

# --- Install binary --------------------------------------------------------------------
Write-Host "[2/4] Installing binary..."
New-Item -ItemType Directory -Force -Path $InstallDir, $DataDir, $ReposDir | Out-Null
if ($isUpdate -and (Test-Path $Binary)) {
    Copy-Item $Binary "$Binary.old" -Force
}
Move-Item $tmpBin $Binary -Force
& $Binary --version *> $null
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: installed binary is not runnable. Restoring previous version..." -ForegroundColor Red
    if (Test-Path "$Binary.old") { Move-Item "$Binary.old" $Binary -Force }
    exit 1
}
$newVersion = (& $Binary --version 2>$null)

# --- Service -----------------------------------------------------------------------------
Write-Host "[3/4] Configuring Windows service (port $Port)..."
$binArgs = "`"$Binary`" -port $Port"
if (-not $svc) {
    New-Service -Name $ServiceName -BinaryPathName $binArgs `
        -DisplayName 'MyGit' -Description 'MyGit - self-hosted Git platform' `
        -StartupType Automatic | Out-Null
} else {
    sc.exe config $ServiceName binPath= $binArgs | Out-Null
    Set-Service -Name $ServiceName -StartupType Automatic
}
# auto-restart on failure (equivalent of Restart=always in the systemd unit)
sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/5000/restart/5000 | Out-Null

# Environment for the service account (equivalent of Environment= in the unit):
# written to HKLM\SYSTEM\CurrentControlSet\Services\<name>\Environment (REG_MULTI_SZ).
$svcKey = "HKLM:\SYSTEM\CurrentControlSet\Services\$ServiceName"
Set-ItemProperty -Path $svcKey -Name Environment -PropertyType MultiString -Value @(
    "MYGIT_BASE_DIR=$DataDir",
    "MYGIT_REPOS_ROOT=$ReposDir",
    "MYGIT_DB_PATH=$DataDir\mygit.db"
)

# restrict ACLs: SYSTEM and Administrators only on data dir (contains DB + repos)
icacls $DataDir /inheritance:r /grant:r 'SYSTEM:(OI)(CI)F' 'Administrators:(OI)(CI)F' | Out-Null

Start-Service -Name $ServiceName

# --- Health check ---------------------------------------------------------------------------
Write-Host -NoNewline "Waiting for MyGit on port $Port to become healthy..."
$healthy = $false
foreach ($i in 1..15) {
    try {
        Invoke-WebRequest -Uri "http://localhost:$Port/api/v1/health" -UseBasicParsing -TimeoutSec 2 | Out-Null
        Write-Host " OK"; $healthy = $true; break
    } catch {
        Write-Host -NoNewline "."; Start-Sleep -Seconds 1
    }
}
if (-not $healthy) {
    Write-Host " FAILED — is port $Port free? (another service may be using it)"
    exit 1
}

Write-Host "[4/4] Done."
Write-Host ""
if ($isUpdate) {
    Write-Host "MyGit updated: ${oldVersion} -> ${newVersion}"
    Write-Host "Config, repositories and users preserved."
} else {
    Write-Host "MyGit installed. Version: $newVersion"
}
Write-Host ""
Write-Host "Dashboard: http://localhost:$Port/"
Write-Host "Зареєструйте ПЕРШИЙ обліковий запис — він стане власником (superuser)."
Write-Host ""
if (Test-Path "$Binary.old") { Write-Host "Previous binary kept at: $Binary.old" }
Write-Host "Installed version: $newVersion"
