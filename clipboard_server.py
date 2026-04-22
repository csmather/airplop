#!/usr/bin/env python3
"""
airplop
----------------------
Run this in WSL2. Your PC controls it; phones just open a URL in Safari.

Install deps:
    pip install flask qrcode[pil] --break-system-packages

Run:
    python3 clipboard_server.py

First-time Windows setup (run setup_windows.ps1 as Admin in PowerShell).
"""

import base64
import io
import json
import queue
import socket
import subprocess
import sys
import threading
import time

from flask import Flask, Response, jsonify, render_template_string, request

PORT = 8765
app = Flask(__name__)

# ── shared state ─────────────────────────────────────────────────────────────
state = {"text": "", "ts": 0}
subscribers: list[queue.Queue] = []
state_lock = threading.Lock()


# ── helpers ───────────────────────────────────────────────────────────────────


def get_windows_ip() -> str | None:
    """Ask PowerShell for the Windows LAN IP (works from WSL2)."""
    try:
        ps = (
            "Get-NetIPAddress -AddressFamily IPv4 "
            "| Where-Object { $_.IPAddress -notlike '127.*' "
            "-and $_.IPAddress -notlike '172.*' "
            "-and $_.IPAddress -notlike '169.*' } "
            "| Sort-Object PrefixLength "
            "| Select-Object -Last 1 "
            "| Select-Object -ExpandProperty IPAddress"
        )
        result = subprocess.run(
            [
                "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
                "-NoProfile",
                "-Command",
                ps,
            ],
            capture_output=True,
            text=True,
            timeout=5,
        )
        ip = result.stdout.strip()
        return ip if ip else None
    except Exception:
        return None


def make_qr_base64(url: str) -> str | None:
    try:
        import qrcode  # type: ignore

        img = qrcode.make(url)
        buf = io.BytesIO()
        img.save(buf, format="PNG")
        return base64.b64encode(buf.getvalue()).decode()
    except Exception:
        return None


def notify_subscribers(data: dict) -> None:
    with state_lock:
        dead = []
        for q in subscribers:
            try:
                q.put_nowait(data)
            except queue.Full:
                dead.append(q)
        for q in dead:
            subscribers.remove(q)


# ── HTML templates ─────────────────────────────────────────────────────────────

SENDER_HTML = """<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>airplop — Send</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #0f0f11;
      color: #e8e8ea;
      min-height: 100vh;
      display: flex;
      align-items: flex-start;
      justify-content: center;
      padding: 40px 20px;
    }
    .card {
      background: #1a1a1f;
      border: 1px solid #2a2a32;
      border-radius: 16px;
      padding: 32px;
      width: 100%;
      max-width: 600px;
      display: flex;
      flex-direction: column;
      gap: 24px;
    }
    h1 { font-size: 1.4rem; font-weight: 600; color: #fff; }
    h1 span { color: #7c6af7; }
    textarea {
      width: 100%;
      height: 160px;
      background: #0f0f11;
      border: 1px solid #2a2a32;
      border-radius: 10px;
      color: #e8e8ea;
      font-size: 1rem;
      padding: 14px;
      resize: vertical;
      outline: none;
      transition: border-color .2s;
      font-family: inherit;
    }
    textarea:focus { border-color: #7c6af7; }
    button {
      background: #7c6af7;
      color: #fff;
      border: none;
      border-radius: 10px;
      padding: 13px 28px;
      font-size: 1rem;
      font-weight: 600;
      cursor: pointer;
      transition: background .15s, transform .1s;
      align-self: flex-start;
    }
    button:hover  { background: #6b58e8; }
    button:active { transform: scale(.97); }
    .status {
      font-size: .85rem;
      color: #888;
      min-height: 1.2em;
      transition: color .3s;
    }
    .status.ok  { color: #4caf87; }
    .status.err { color: #e05252; }
    .qr-section {
      display: flex;
      flex-direction: column;
      gap: 10px;
      border-top: 1px solid #2a2a32;
      padding-top: 20px;
    }
    .qr-label { font-size: .8rem; color: #888; text-transform: uppercase; letter-spacing: .05em; }
    .qr-url {
      font-family: "Courier New", monospace;
      font-size: .9rem;
      color: #7c6af7;
      word-break: break-all;
    }
    .qr-img { border-radius: 10px; background: #fff; padding: 8px; width: 140px; height: 140px; }
    .row { display: flex; align-items: center; gap: 20px; flex-wrap: wrap; }
    kbd {
      background: #2a2a32;
      border-radius: 4px;
      padding: 2px 6px;
      font-size: .8rem;
      color: #aaa;
    }
  </style>
</head>
<body>
<div class="card">
  <h1>air<span>plop</span></h1>

  <textarea id="txt" placeholder="Paste a link or type something…" autofocus></textarea>

  <div class="row">
    <button onclick="send()">Send <kbd>Ctrl+↵</kbd></button>
    <span class="status" id="status"></span>
  </div>

  <div class="qr-section">
    <span class="qr-label">Phone viewer URL — scan or open in Safari</span>
    <div class="row">
      {% if qr_b64 %}
      <img class="qr-img" src="data:image/png;base64,{{ qr_b64 }}" alt="QR code">
      {% endif %}
      <span class="qr-url">{{ viewer_url }}</span>
    </div>
  </div>
</div>

<script>
  const txt    = document.getElementById('txt');
  const status = document.getElementById('status');

  function setStatus(msg, cls) {
    status.textContent = msg;
    status.className = 'status ' + (cls || '');
    if (cls === 'ok') setTimeout(() => { status.textContent = ''; status.className = 'status'; }, 2000);
  }

  async function send() {
    const text = txt.value.trim();
    if (!text) return;
    try {
      const res = await fetch('/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
      });
      if (res.ok) { setStatus('Sent ✓', 'ok'); }
      else        { setStatus('Error ' + res.status, 'err'); }
    } catch(e) {
      setStatus('Network error', 'err');
    }
  }

  document.addEventListener('keydown', e => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') send();
  });
</script>
</body>
</html>
"""

RECEIVER_HTML = """<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <title>airplop</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #0f0f11;
      color: #e8e8ea;
      min-height: 100svh;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: flex-start;
      padding: 48px 20px env(safe-area-inset-bottom, 20px);
    }
    .card {
      background: #1a1a1f;
      border: 1px solid #2a2a32;
      border-radius: 20px;
      padding: 28px 24px;
      width: 100%;
      max-width: 480px;
      display: flex;
      flex-direction: column;
      gap: 20px;
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    h1 { font-size: 1.1rem; font-weight: 600; color: #fff; }
    .dot {
      width: 8px; height: 8px;
      border-radius: 50%;
      background: #4caf87;
      box-shadow: 0 0 6px #4caf87;
      transition: background .3s;
    }
    .dot.off { background: #555; box-shadow: none; }
    .content {
      background: #0f0f11;
      border: 1px solid #2a2a32;
      border-radius: 12px;
      padding: 18px;
      min-height: 100px;
      font-size: 1.05rem;
      line-height: 1.6;
      word-break: break-word;
      white-space: pre-wrap;
      color: #e8e8ea;
    }
    .content.empty { color: #555; font-style: italic; }
    .actions {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
    }
    button {
      flex: 1;
      min-width: 120px;
      background: #2a2a32;
      color: #e8e8ea;
      border: none;
      border-radius: 10px;
      padding: 13px;
      font-size: .95rem;
      font-weight: 600;
      cursor: pointer;
      transition: background .15s, transform .1s;
    }
    button:active { transform: scale(.96); }
    .btn-primary { background: #7c6af7; color: #fff; }
    .btn-primary:hover { background: #6b58e8; }
    .btn-link { background: #1e3a2f; color: #4caf87; }
    .flash {
      position: fixed; top: 20px; left: 50%; transform: translateX(-50%);
      background: #7c6af7; color: #fff;
      padding: 10px 22px;
      border-radius: 40px;
      font-size: .9rem;
      font-weight: 600;
      opacity: 0;
      transition: opacity .2s;
      pointer-events: none;
      white-space: nowrap;
    }
    .flash.show { opacity: 1; }
  </style>
</head>
<body>
<div class="card">
  <div class="header">
    <h1>Clipboard</h1>
    <div class="dot off" id="dot" title="Live connection"></div>
  </div>

  <div class="content empty" id="content">Nothing yet — waiting…</div>

  <div class="actions" id="actions">
    <button class="btn-primary" onclick="copyText()">Copy</button>
  </div>
</div>

<div class="flash" id="flash">Copied!</div>

<script>
  let currentText = '';
  const contentEl = document.getElementById('content');
  const actionsEl = document.getElementById('actions');
  const dotEl     = document.getElementById('dot');
  const flashEl   = document.getElementById('flash');

  function isUrl(s) {
    try { return ['http:', 'https:'].includes(new URL(s.trim()).protocol); }
    catch { return false; }
  }

  function render(text) {
    currentText = text;
    if (!text) {
      contentEl.textContent = 'Nothing yet — waiting…';
      contentEl.className = 'content empty';
      actionsEl.innerHTML = '<button class="btn-primary" onclick="copyText()">Copy</button>';
      return;
    }
    contentEl.textContent = text;
    contentEl.className = 'content';

    let btns = '<button class="btn-primary" onclick="copyText()">Copy</button>';
    if (isUrl(text)) {
      btns += `<button class="btn-link" onclick="openLink()">Open Link</button>`;
    }
    actionsEl.innerHTML = btns;
  }

  function copyText() {
    if (!currentText) return;
    navigator.clipboard.writeText(currentText).then(() => {
      flashEl.classList.add('show');
      setTimeout(() => flashEl.classList.remove('show'), 1600);
    });
  }

  function openLink() {
    if (currentText) window.open(currentText.trim(), '_blank');
  }

  // ── Server-Sent Events ──────────────────────────────────────────────────
  function connect() {
    const es = new EventSource('/stream');

    es.onopen = () => { dotEl.className = 'dot'; };

    es.onmessage = e => {
      try {
        const data = JSON.parse(e.data);
        render(data.text || '');
      } catch {}
    };

    es.onerror = () => {
      dotEl.className = 'dot off';
      es.close();
      setTimeout(connect, 3000);   // auto-reconnect
    };
  }

  connect();
</script>
</body>
</html>
"""


# ── routes ─────────────────────────────────────────────────────────────────────


@app.route("/")
def sender():
    windows_ip = app.config.get("WINDOWS_IP")
    viewer_url = (
        f"http://{windows_ip}:{PORT}/view"
        if windows_ip
        else f"http://<your-windows-ip>:{PORT}/view"
    )
    qr_b64 = make_qr_base64(viewer_url) if windows_ip else None
    return render_template_string(SENDER_HTML, viewer_url=viewer_url, qr_b64=qr_b64)


@app.route("/view")
def receiver():
    return render_template_string(RECEIVER_HTML)


@app.route("/update", methods=["POST"])
def update():
    data = request.get_json(silent=True) or {}
    text = str(data.get("text", "")).strip()
    with state_lock:
        state["text"] = text
        state["ts"] = time.time()
    notify_subscribers({"text": text, "ts": state["ts"]})
    return jsonify({"ok": True})


@app.route("/stream")
def stream():
    """SSE endpoint — each phone keeps this connection open."""
    q: queue.Queue = queue.Queue(maxsize=10)
    with state_lock:
        subscribers.append(q)
        # send current state immediately on connect
        current = dict(state)

    def event_generator():
        # push current state right away
        yield f"data: {json.dumps(current)}\n\n"
        try:
            while True:
                try:
                    payload = q.get(timeout=25)
                    yield f"data: {json.dumps(payload)}\n\n"
                except queue.Empty:
                    yield ": heartbeat\n\n"  # keep-alive
        finally:
            with state_lock:
                if q in subscribers:
                    subscribers.remove(q)

    return Response(
        event_generator(),
        content_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )


# ── main ───────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print("\n🔌  airplop")
    print("─" * 40)

    windows_ip = get_windows_ip()
    if windows_ip:
        print(f"  Windows IP   : {windows_ip}")
        print(f"  Sender (PC)  : http://localhost:{PORT}")
        print(f"  Viewer (phone): http://{windows_ip}:{PORT}/view")
        app.config["WINDOWS_IP"] = windows_ip
    else:
        print("  ⚠  Could not detect Windows IP automatically.")
        print(f"     Run `ipconfig` in Windows, find your LAN IP,")
        print(f"     then open: http://<your-ip>:{PORT}/view on the phones.")
        print(f"  Sender (PC)  : http://localhost:{PORT}")
        app.config["WINDOWS_IP"] = None

    print("─" * 40)
    print("  Ctrl+C to stop\n")

    app.run(host="0.0.0.0", port=PORT, threaded=True)
