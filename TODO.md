# TODO

- [x] Stable URL for phones — mDNS `airplop.local` via zeroconf
- [x] Auto-rerun port forwarding on boot — Scheduled Task in `setup_windows.ps1`
- [ ] **(high)** Make it a "living document" — no separate `/view` page. Either one conjoined send+receive box where everything flows through, or side-by-side send and receive boxes on the same page. Idea not refined yet.
- [ ] Send history — keep last N items, let the phone scroll back
- [ ] Bidirectional phone → PC auto-copy — tiny POST endpoint from the phone, pipe text into Windows clipboard via `clip.exe`
- [ ] File/image support — drag-and-drop on the sender page, phone downloads it
- [ ] *(optional, low prio)* Auto-start the WSL server on boot — systemd user unit + `loginctl enable-linger`, completes the zero-touch reboot story

## Language rewrites (for learning)

- [x] Port to **Go** — `net/http` for SSE, `grandcat/zeroconf` for mDNS. Runs in WSL2 (mirrored mode); the cross-compile-to-`.exe` goal didn't pan out because Go's mDNS libs are flaky on native Windows multicast (see "rainy day" item below).
- [ ] *(later/maybe)* Port to **Rust** — `axum` + `mdns-sd`. Overkill for the scale, but a bounded project to learn ownership/borrow checker without deadline pressure.
- [ ] *(later/maybe)* Port to **Node/TypeScript** — Express + an mDNS lib. Smallest delta from Python, smallest learning dividend; only worth it for JS/TS reps.
- [ ] *(wildcard)* Port to **Elixir (Phoenix + LiveView)** — realtime push is BEAM's sweet spot. Steepest curve, but the actor model is genuinely different from everything else here.

## Rainy-day Go project

- [ ] *(low prio, learning-only)* Make Go port work as a native Windows `.exe`. Replace `grandcat/zeroconf` with platform-tagged mDNS:
  - `mdns_linux.go` — keep the current `grandcat/zeroconf` path
  - `mdns_windows.go` — call Windows's built-in mDNS responder via `dnsapi.dll` (`DnsServiceRegister`, available since Win10 1809) using `golang.org/x/sys/windows`. No third-party Go lib, no firewall popup, no fragile multicast-from-Go — Windows does the multicast itself. ~150 lines of syscall wrapping.
  - Bonus: drops the WSL requirement entirely and gives you a single-file double-clickable distribution.
