# airplop

Send text and links from your PC to iPhones on the same local network. No apps required — just Safari.

## How it works

A Flask server runs in WSL2 and pushes clipboard content to connected phones via Server-Sent Events. You type/paste on the PC, phones see it instantly.

- **Sender** (PC): `http://localhost:8765` — paste text, hit Send
- **Receiver** (phone): `http://<your-windows-ip>:8765/view` — opens in Safari, auto-updates via SSE
- QR code on the sender page for easy phone setup

## Setup

### Requirements

- Windows 11 with WSL2 (Ubuntu)
- Python 3.10+
- Phone on the same WiFi network

### Install

```bash
pip install flask 'qrcode[pil]'
```

### Windows networking (one-time, as Admin)

Port-forwards from Windows to WSL2 and opens the firewall:

```powershell
# Run as Administrator
.\setup_windows.ps1
```

> WSL2 gets a new IP on each reboot — re-run this script after rebooting.

### Run

```bash
python3 clipboard_server.py
```

Open `http://localhost:8765` on your PC, scan the QR code with your phone.
