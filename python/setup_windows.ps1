# setup_windows.ps1
# Run ONCE as Administrator. Idempotent — safe to re-run.
#
# Does:
#   - Ensures the Windows firewall rule exists (port 8765, Private profile)
#   - If WSL2 is NOT in mirrored networking mode, sets up netsh portproxy as a fallback
#   - Registers a Scheduled Task that re-runs this script at logon (admin, silent)
#     so the port forward (if needed) survives reboots without manual re-elevation
#
# Usage:
#   Right-click PowerShell -> "Run as Administrator", cd to this folder, then:
#   .\setup_windows.ps1

$ErrorActionPreference = "Stop"
$PORT = 8765
$TaskName = "airplop-portforward"
$ScriptPath = $MyInvocation.MyCommand.Path

function Test-Admin {
    $id = [System.Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object System.Security.Principal.WindowsPrincipal($id)
    return $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-Admin)) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

# -- Detect WSL2 mirrored networking mode -------------------------------------
$wslconfig = Join-Path $env:USERPROFILE ".wslconfig"
$mirrored = $false
if (Test-Path $wslconfig) {
    $content = Get-Content $wslconfig -Raw
    if ($content -match '(?im)^\s*networkingMode\s*=\s*mirrored\s*$') {
        $mirrored = $true
    }
}

if ($mirrored) {
    Write-Host "WSL2 mirrored networking detected - port forwarding not needed." -ForegroundColor Green
} else {
    Write-Host "WSL2 mirrored networking NOT detected - will install netsh portproxy." -ForegroundColor Yellow
    Write-Host "  (Tip: add 'networkingMode = mirrored' under [wsl2] in $wslconfig," -ForegroundColor DarkGray
    Write-Host "   then 'wsl --shutdown', to skip this step on future runs.)" -ForegroundColor DarkGray
}

# -- Firewall rule (always needed) --------------------------------------------
Remove-NetFirewallRule -DisplayName "airplop" -ErrorAction SilentlyContinue
New-NetFirewallRule `
    -DisplayName "airplop" `
    -Direction Inbound `
    -Protocol TCP `
    -LocalPort $PORT `
    -Action Allow `
    -Profile Private | Out-Null
Write-Host "Firewall rule ensured for TCP $PORT (Private profile)." -ForegroundColor Green

# -- Port forward (only if NOT mirrored) --------------------------------------
if (-not $mirrored) {
    $wslIp = (wsl hostname -I).Trim().Split(" ")[0]
    if (-not $wslIp) {
        Write-Error "Could not detect WSL2 IP. Is WSL running?"
        exit 1
    }
    netsh interface portproxy delete v4tov4 listenport=$PORT listenaddress=0.0.0.0 2>$null | Out-Null
    netsh interface portproxy add v4tov4 `
        listenport=$PORT `
        listenaddress=0.0.0.0 `
        connectport=$PORT `
        connectaddress=$wslIp | Out-Null
    Write-Host "Port forward installed: 0.0.0.0:${PORT} -> ${wslIp}:${PORT}" -ForegroundColor Green
} else {
    # Clean up any stale portproxy from pre-mirrored setup runs
    netsh interface portproxy delete v4tov4 listenport=$PORT listenaddress=0.0.0.0 2>$null | Out-Null
}

# -- Scheduled Task: re-run this script at logon with admin -------------------
$action = New-ScheduledTaskAction `
    -Execute "powershell.exe" `
    -Argument "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$ScriptPath`""
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal `
    -UserId "$env:USERDOMAIN\$env:USERNAME" `
    -LogonType Interactive `
    -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -ExecutionTimeLimit (New-TimeSpan -Minutes 5)

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $action `
    -Trigger $trigger `
    -Principal $principal `
    -Settings $settings `
    -Description "Re-establishes airplop network setup after boot/logon." | Out-Null
Write-Host "Scheduled Task '$TaskName' registered - runs at logon with admin." -ForegroundColor Green

Write-Host ""
Write-Host "Setup complete. Start the server in WSL2:" -ForegroundColor Yellow
Write-Host "  python3 clipboard_server.py" -ForegroundColor White
Write-Host ""
Write-Host "Phones open: http://airplop.local:$PORT/view" -ForegroundColor Cyan
