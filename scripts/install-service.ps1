<#
.SYNOPSIS
    Registers SSH Backup Manager as a Windows Service (Automatic startup).

.DESCRIPTION
    Must be run from an elevated (Administrator) PowerShell prompt. Copies
    no files; it registers the service against the exe path you pass in
    (or the default build output location).

.PARAMETER ExePath
    Path to vpsbackupservice.exe. Defaults to the server/bin/ build output
    relative to the repository root.

.EXAMPLE
    .\scripts\install-service.ps1
    .\scripts\install-service.ps1 -ExePath "C:\Program Files\VPSBackupManager\vpsbackupservice.exe"
#>
[CmdletBinding()]
param(
    [string]$ExePath
)

$ErrorActionPreference = "Stop"

if (-not $ExePath) {
    $ScriptRoot = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
    $ExePath = Join-Path $ScriptRoot "..\server\bin\vpsbackupservice.exe"
}

$ServiceName = "SSH Backup Manager Service"
$DisplayName = "SSH Backup Manager Service"
$Description = "Runs scheduled VPS backups over SSH. See https://github.com/nyver/sshbackup for documentation."

$resolvedExe = (Resolve-Path -Path $ExePath -ErrorAction Stop).Path

if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
    Write-Host "Service '$ServiceName' is already registered. Run uninstall-service.ps1 first if you want to re-register it." -ForegroundColor Yellow
    exit 1
}

New-Service `
    -Name $ServiceName `
    -BinaryPathName "`"$resolvedExe`"" `
    -DisplayName $DisplayName `
    -Description $Description `
    -StartupType Automatic

Write-Host "Registered '$ServiceName' (Automatic startup) using $resolvedExe" -ForegroundColor Green
Write-Host "Start it with: .\scripts\start-service.ps1"
