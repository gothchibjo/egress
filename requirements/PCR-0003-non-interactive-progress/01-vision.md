---
pcr: PCR-0003
title: Non-interactive progress indication
slug: non-interactive-progress
status: frozen
requester: gothchibjo (repo owner)
channel: chat paste
received: 2026-11-12
source_language: en
artifact_language: en
participants: [analyst, customer]
---

# PART A — Verbatim Vision (As-Is)

> Immutable section: never edited after registration. Byte-for-byte as
> received.

The utility shall display execution progress indication when launched in
non-interactive mode.

We already have progress in the interactive mode in the footer ("running
tcp.anchor.apple-captive.53 (13/17) 2s"), and we can do the same instead of
building separate progress indicators.

# PART B — Clarification Log (append-only)

> Never rewritten; only appended to by req-elicitation.

### CL-1 (answered) — 2026-11-12

> Q1 — Output stream: where should the progress indication go? ➡️ stderr, so
> stdout stays pure JSON and can be piped.
>
> Q2 — When to show it: always in non-interactive mode, or only when stderr is a
> terminal? ➡️ only when stderr is a terminal, so CI logs and redirected output
> stay clean.
>
> Q3 — Format and cadence: reuse the interactive footer text (`running
> <check-id> (<index>/<total>) <elapsed>`) rewritten in place about once per
> second? ➡️ yes, rewritten with a carriage return.
>
> Q4 — Start: show the running check immediately, or only once a check exceeds
> the slow-check threshold used by the dashboard? ➡️ show it immediately; the
> complaint is the silent first seconds.
>
> Q5 — Completion: erase the progress line before the JSON is written, or leave
> a final summary line? ➡️ erase it so the terminal is left as if only the JSON
> ran.
>
> Q6 — Cancellation: on Ctrl-C, print a final cancelled state and still emit the
> partial JSON with the documented exit code? ➡️ yes.
>
> CUSTOMER (relay): 1–6. Yes.
