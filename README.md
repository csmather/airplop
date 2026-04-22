# airplop

Send text and links from your PC to iPhones on the same local network. No apps required — just Safari.

## How it works

A Flask server runs in WSL2 and pushes clipboard content to connected phones via Server-Sent Events. You type/paste on the PC, phones see it instantly.

- **Sender** (PC): `http://localhost:8765` — paste text, hit Send
- **Receiver** (phone): `http://airplop.local:8765/view` — opens in Safari, auto-updates via SSE
- QR code on the sender page for easy phone setup
- Phone bookmark survives reboots: the server advertises `airplop.local` over mDNS, so the URL stays stable even if the LAN IP changes

## Setup

### Requirements

- Windows 11 with WSL2 (Ubuntu)
- Python 3.10+
- Phone on the same WiFi network

### Install

```bash
pip install flask 'qrcode[pil]' zeroconf
```

### Windows networking (one-time, as Admin)

Opens the firewall and registers a Scheduled Task so the network setup
re-runs automatically at every logon:

```powershell
# Run as Administrator (ONCE — the script self-installs a logon task)
.\setup_windows.ps1
```

The script auto-detects WSL2 mirrored networking mode (recommended) and skips
port forwarding when it's not needed. To enable mirrored mode, add this to
`%USERPROFILE%\.wslconfig`:

```ini
[wsl2]
networkingMode = mirrored
```

then run `wsl --shutdown`.

### Run

```bash
python3 clipboard_server.py
```

Open `http://localhost:8765` on your PC, scan the QR code with your phone.
The phone bookmark is `http://airplop.local:8765/view` — stable across reboots.
