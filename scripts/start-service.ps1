<#
.SYNOPSIS
    Starts the VPS Backup Manager Service. Requires an elevated prompt.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$ServiceName = "VPS Backup Manager Service"

Start-Service -Name $ServiceName
Get-Service -Name $ServiceName | Format-Table -AutoSize
