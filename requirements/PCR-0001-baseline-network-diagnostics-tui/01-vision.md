---
pcr: PCR-0001
title: Baseline network diagnostics and TUI shell
slug: baseline-network-diagnostics-tui
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

A utility for performing basic network diagnostics to detect blocks. The UX is roughly as follows: we launch the utility, the utility performs basic network diagnostics and displays the results.

The language is Go. With a UI, but all display of results is in the terminal as well. Keyboard controls.

# PART B — Clarification Log (append-only)

> Never rewritten; only appended to by req-elicitation.

### CL-1 (answered) — 2026-11-10

> Q1 — baseline scope: Which checks must the automatic baseline run include? ➡️ Full recommended set: interfaces/default route, gateway reachability, DNS config + canary resolution, proxy env, system DNS vs DoH, ICMP to gateway/anchors, TCP 443/53 to anchors, captive-portal HTTP canary, MTU/PMTU probe, latency/jitter/loss sample.
> Q2 — startup flow: What happens when the utility starts? ➡️ Auto-run the baseline, then show a dashboard; target mode and re-runs start from the dashboard.
> Q3 — runtime budget: How long may the automatic baseline take on a typical network? ➡️ ≤30 seconds, with progress shown live.
> Q4 — privileges: How should privileges be handled? ➡️ Unprivileged by default; richer ICMP/raw tracing only when the user elevates. No sudo prompt on startup.
> Q5 — platforms: Which platforms must v1 support? ➡️ macOS and Linux.
> Q6 — output modes: Besides the interactive TUI, should there be non-interactive output? ➡️ TUI plus `--json` and a non-interactive mode.
> Q7 — privacy: May the utility call external services or upload results? ➡️ Local only: no telemetry, no uploads, no third-party enrichment APIs.
> Q8 — baseline anchors: Which neutral anchors should the baseline use? ➡️ Curated multi-vendor set: Cloudflare 1.1.1.1, Google 8.8.8.8, Apple captive-portal canary, example.com.

> CUSTOMER (relay): Full recommended set; auto-run then dashboard; ≤30 seconds; unprivileged by default with optional elevation; macOS + Linux; TUI + JSON/non-interactive; local only, no external APIs; curated multi-vendor anchors.

### CL-2 (answered) — 2026-11-10

> Q1 — DoH exception: The chosen baseline includes "system DNS vs DoH", which uses public resolvers. How does that square with "local only, no external APIs"? ➡️ The "local only" rule forbids telemetry, uploads and enrichment APIs, but diagnostic probes to public resolvers/anchors (Cloudflare/Google DoH, captive-portal canary) are allowed because they are the measurement itself.

> CUSTOMER (relay): Diagnostic probes to public resolvers and anchors are allowed; telemetry, uploads and enrichment APIs remain forbidden.

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
