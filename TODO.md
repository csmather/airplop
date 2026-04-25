# TODO

- [x] Stable URL for phones — just bookmark the LAN IP. Tried mDNS first; Go's libs are bad and the protocol-vs-payoff ratio is terrible for a one-user gadget. Working hand-rolled responder is parked on the `mdns` branch if curiosity strikes again.
- [x] Auto-rerun port forwarding on boot — Scheduled Task in `setup_windows.ps1`
- [ ] **(high)** Make it a "living document" — no separate `/view` page. Either one conjoined send+receive box where everything flows through, or side-by-side send and receive boxes on the same page. Idea not refined yet.
- [ ] Send history — keep last N items, let the phone scroll back
- [ ] Bidirectional phone → PC auto-copy — tiny POST endpoint from the phone, pipe text into Windows clipboard via `clip.exe`
- [ ] File/image support — drag-and-drop on the sender page, phone downloads it
- [ ] *(optional, low prio)* Auto-start the WSL server on boot — systemd user unit + `loginctl enable-linger`, completes the zero-touch reboot story

## Language rewrites (for learning)

- [x] Port to **Go** — `net/http` for SSE, no external deps. Runs in WSL2 (mirrored mode). With mDNS gone, a native Windows `.exe` via `GOOS=windows go build` is now trivial — just haven't bothered.
- [ ] *(later/maybe)* Port to **Rust** — `axum` + `mdns-sd`. Overkill for the scale, but a bounded project to learn ownership/borrow checker without deadline pressure.
- [ ] *(later/maybe)* Port to **Node/TypeScript** — Express + an mDNS lib. Smallest delta from Python, smallest learning dividend; only worth it for JS/TS reps.
- [ ] *(wildcard)* Port to **Elixir (Phoenix + LiveView)** — realtime push is BEAM's sweet spot. Steepest curve, but the actor model is genuinely different from everything else here.

