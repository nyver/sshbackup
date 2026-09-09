<#
.SYNOPSIS
    Stops and removes the SSH Backup Manager Windows Service registration.
    Requires an elevated prompt.

.DESCRIPTION
    Only unregisters the service; it does not delete
    %ProgramData%\VPSBackupManager (database, archives, logs, DPAPI
    secrets) or the installed binaries. Remove those manually if you want
    a full uninstall.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$ServiceName = "SSH Backup Manager Service"

$svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $svc) {
    Write-Host "Service '$ServiceName' is not registered." -ForegroundColor Yellow
    exit 0
}

if ($svc.Status -ne "Stopped") {
    Stop-Service -Name $ServiceName
    $svc.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(150))
}

# sc.exe delete is used because Remove-Service requires PowerShell 6+;
# sc.exe ships with every supported Windows version.
sc.exe delete $ServiceName | Out-Null

Write-Host "Removed service '$ServiceName'." -ForegroundColor Green
Write-Host "Data directory %ProgramData%\VPSBackupManager was left in place; remove it manually if desired."
