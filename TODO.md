# TODO

- [x] Stable URL for phones — mDNS `airplop.local` via zeroconf
- [x] Auto-rerun port forwarding on boot — Scheduled Task in `setup_windows.ps1`
- [ ] **(high)** Make it a "living document" — no separate `/view` page. Either one conjoined send+receive box where everything flows through, or side-by-side send and receive boxes on the same page. Idea not refined yet.
- [ ] Send history — keep last N items, let the phone scroll back
- [ ] Bidirectional phone → PC auto-copy — tiny POST endpoint from the phone, pipe text into Windows clipboard via `clip.exe`
- [ ] File/image support — drag-and-drop on the sender page, phone downloads it
- [ ] *(optional, low prio)* Auto-start the WSL server on boot — systemd user unit + `loginctl enable-linger`, completes the zero-touch reboot story
