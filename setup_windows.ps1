# setup_windows.ps1
# Run this ONCE in PowerShell as Administrator.
# It forwards port 8765 from Windows -> WSL2 and opens the firewall.
#
# Usage:
#   Right-click PowerShell -> "Run as Administrator"
#   cd to wherever you saved this, then:
#   .\setup_windows.ps1

$PORT = 8765

# Get current WSL2 IP
$wslIp = (wsl hostname -I).Trim().Split(" ")[0]

if (-not $wslIp) {
    Write-Error "Could not detect WSL2 IP. Make sure WSL2 is running."
    exit 1
}

Write-Host "WSL2 IP: $wslIp" -ForegroundColor Cyan

# Remove any old rule for this port (clean slate)
netsh interface portproxy delete v4tov4 listenport=$PORT listenaddress=0.0.0.0 2>$null

# Add port forward: Windows:8765 -> WSL2:8765
netsh interface portproxy add v4tov4 `
    listenport=$PORT `
    listenaddress=0.0.0.0 `
    connectport=$PORT `
    connectaddress=$wslIp

Write-Host "Port forward added: 0.0.0.0:${PORT} -> ${wslIp}:${PORT}" -ForegroundColor Green

# Remove old firewall rule if it exists
Remove-NetFirewallRule -DisplayName "airplop" -ErrorAction SilentlyContinue

# Add inbound firewall rule
New-NetFirewallRule `
    -DisplayName "airplop" `
    -Direction Inbound `
    -Protocol TCP `
    -LocalPort $PORT `
    -Action Allow `
    -Profile Private | Out-Null

Write-Host "Firewall rule added for port $PORT (Private networks)" -ForegroundColor Green
Write-Host ""
Write-Host "All done! Start the server in WSL2 with:" -ForegroundColor Yellow
Write-Host "  python3 clipboard_server.py" -ForegroundColor White
Write-Host ""
Write-Host "NOTE: WSL2 gets a new IP on each reboot." -ForegroundColor DarkYellow
Write-Host "      Re-run this script after rebooting if phones can't connect." -ForegroundColor DarkYellow
