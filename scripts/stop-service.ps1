<#
.SYNOPSIS
    Stops the SSH Backup Manager Service gracefully. Requires an elevated
    prompt.

.DESCRIPTION
    Sends a normal stop request; the service itself bounds how long it
    waits for active runs' cleanup actions before exiting (see
    server/cmd/vpsbackupservice's shutdownBudget).
#>
[CmdletBinding()]
param(
    [int]$TimeoutSeconds = 150
)

$ErrorActionPreference = "Stop"
$ServiceName = "SSH Backup Manager Service"

Stop-Service -Name $ServiceName
try {
    (Get-Service -Name $ServiceName).WaitForStatus("Stopped", [TimeSpan]::FromSeconds($TimeoutSeconds))
    Write-Host "Service stopped." -ForegroundColor Green
} catch {
    Write-Warning "Service did not report Stopped within $TimeoutSeconds seconds; check the service log."
}
