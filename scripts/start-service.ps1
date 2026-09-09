<#
.SYNOPSIS
    Starts the SSH Backup Manager Service. Requires an elevated prompt.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$ServiceName = "SSH Backup Manager Service"

Start-Service -Name $ServiceName
Get-Service -Name $ServiceName | Format-Table -AutoSize
