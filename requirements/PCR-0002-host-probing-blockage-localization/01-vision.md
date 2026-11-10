---
pcr: PCR-0002
title: Interactive host probing and blockage localization
slug: host-probing-blockage-localization
status: technical
requester: gothchibjo (repo owner)
channel: chat paste
received: 2026-11-10
source_language: en
artifact_language: en
participants: [analyst, customer]
---

# PART A — Verbatim Vision (As-Is)

> Immutable section: never edited after registration. Byte-for-byte as
> received.

Next, you can select one of the preset hosts or IPs, or enter your own host or IP, ping them, perform traceroute and other diagnostics to understand at which step packets are being dropped, and if possible determine the type of blocking. It is worth reviewing existing solutions and selecting the necessary functionality from them to implement in our application.

# PART B — Clarification Log (append-only)

> Never rewritten; only appended to by req-elicitation.

### CL-1 (answered) — 2026-11-10

> Q1 — preset catalog: Which hosts should be predefined in the target picker? ➡️ Curated global + RU-relevant: YouTube, Telegram, Discord, GitHub, Google, Cloudflare, Wikipedia, X, Tor, plus neutral anchors 1.1.1.1 and 8.8.8.8.
> Q2 — probe ladder: Which per-target probes must v1 include? ➡️ Full diagnostic ladder: DNS (system vs DoH/authoritative) → TCP connect on multiple ports → TLS ClientHello with real/decoy/no SNI → HTTP GET → ICMP echo; traceroute runs alongside.
> Q3 — traceroute modes: How should traceroute be implemented in v1? ➡️ UDP tracepath-style unprivileged by default; ICMP traceroute when elevated; no packet-capture dependency.
> Q4 — classification: Which blockage-type classification must v1 produce? ➡️ Full taxonomy with confidence: DNS tampering, TCP RST injection, IP drop/blackhole, port block, SNI/TLS filtering, HTTP block/captive portal, throttling, unknown.
> Q5 — custom target: What may the user enter as a custom target? ➡️ Hostname or IP (IPv4/IPv6) with optional port and protocol override; validated inline.
> Q6 — concurrency: How should probes be scheduled? ➡️ Sequential per target; up to 4 targets in parallel.
> Q7 — results & export: How should results be presented and persisted? ➡️ Live TUI cards plus JSON export of the session and a saved run file on demand; no database.
> Q8 — enrichment: Should v1 use external enrichment services? ➡️ None in v1; link out to external looking-glasses only as text.
> Q9 — v1 stretch: Which one, if any, must be pulled into v1 despite the research recommendation? ➡️ Left unanswered; re-posed as CL-2 Q1.

> CUSTOMER (relay): Curated global + RU-relevant presets; full diagnostic ladder; UDP + ICMP-when-elevated traceroute; full taxonomy with confidence; host/IP + optional port; sequential per target with 4 parallel; TUI + JSON export; no enrichment.

### CL-2 (answered) — 2026-11-10

> Q1 — v1 stretch: Which one, if any, must be pulled into v1 despite the research recommendation? ➡️ None: continuous MTR, TCP SYN tracing, distributed measurement, ASN/BGP, throttling quantification and Windows stay out of v1.
> Q2 — proxy/VPN control: Should v1 compare direct vs proxy/VPN paths as a control? ➡️ Detect and show proxy/VPN settings as context only; no automatic A/B comparison in v1.

> CUSTOMER (relay): Keep v1 lean — no MTR, TCP SYN tracing, distributed measurement, ASN/BGP, throttling quantification or Windows; proxy/VPN detected and shown only.

### CL-3 (answered) — 2026-11-10

> Q1 — TUI layout: Which base layout should be used? ➡️ Master-detail plus tabs: dashboard with the path list on the left and evidence on the right; run view with target tabs and adaptive panes; narrow terminals collapse to stacked or tabbed views.
> Q2 — target picker: How should targets be selected? ➡️ Multi-select checkboxes, `space` toggles, `/` filters the catalog, custom target entry on the same screen, `enter` runs the selected targets.
> Q3 — run view: How should run results be shown? ➡️ Target tabs on top; Ladder, Hops and Verdict as inner views; wide terminals show ladder and hops side by side; narrow terminals switch between them.
> Q4 — keys: Which control scheme should be fixed? ➡️ Arrows and `j`/`k` both navigate; `enter` drills in; `esc` goes back; single-letter shortcuts for actions; `?` opens help.
> Q5 — color: Which visual policy should apply? ➡️ Semantic colors plus symbols and text; 256-color and 16-color fallbacks; `NO_COLOR` support; ASCII fallback for non-Unicode terminals; status never depends on color alone.
> Q6 — minimum size: Which minimum terminal size should be supported? ➡️ Normal from 80x24; compact layout from 60x20; below that a clear resize message; below 40x10 refuse to render.
> Q7 — export: How does result export work? ➡️ `e` writes `egress-<timestamp>.json` in the current directory and shows the path in a toast; `E` opens a path prompt if needed.
> Q8 — cancel: How should runs be cancelled and the app exited? ➡️ `x` cancels the current target, `X` cancels all targets, `Ctrl-C` asks to quit and keeps partial results exportable; `esc` only navigates back.

> CUSTOMER (relay): Master-detail + tabs; multi-select picker with search; target tabs and adaptive panes; arrows + vim + shortcuts; color with symbols and fallbacks; 80x24 normal, 60x20 compact, refuse below 40x10; timestamped JSON export with toast; `x` / `X` / `Ctrl-C` cancellation.
