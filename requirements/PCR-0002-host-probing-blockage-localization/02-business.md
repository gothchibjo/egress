---
pcr: PCR-0002
title: Interactive host probing and blockage localization
status: proposed
revised: 2026-11-10
based_on: 01-vision.md
approved_by: null
approved_at: null
revisions: []
---

# Context & Goals

PCR-0002 defines the interactive half of the utility: from the baseline
dashboard, the user selects preset or custom targets, runs a layered diagnostic
ladder, sees where in the path packets stop, and receives a blockage-type
verdict with supporting evidence and confidence. It reuses the TUI shell,
evidence model and privacy rules established in PCR-0001. v1 stays lean:
observation only, no distributed measurement, no enrichment APIs, no packet
capture and no automatic remediation.

# Stakeholders

- **Terminal user** — needs to know which hop or protocol step drops traffic and
  what kind of blockage the evidence supports.
- **Support / helpdesk** — needs exported, reproducible evidence with confidence
  and limitations stated.
- **Catalog maintainer** — owns the preset host list and its addresses.
- **Privacy-sensitive user** — needs every probe destination to be explicit and
  no data to leave the machine beyond the probes.

# Glossary

- **Target** — a hostname or IP address the user chooses to diagnose.
- **Preset** — a built-in target from the curated catalog.
- **Custom target** — a user-entered hostname or IP with optional port and
  protocol override.
- **Ladder** — the ordered set of probes run per target: DNS → TCP connect → TLS
  ClientHello → HTTP → ICMP, with traceroute alongside.
- **Drop hop** — the first hop after which no further response is observed on
  the path.
- **Last responding hop** — the furthest hop that returned a response for the
  probe in question.
- **Blockage type** — the classified hypothesis: DNS tampering, TCP RST
  injection, IP drop/blackhole, port block, SNI/TLS filtering, HTTP
  block/captive portal, throttling or unknown.
- **Confidence** — qualitative strength of a verdict: low, medium or high, tied
  to the evidence available.
- **SNI** — TLS Server Name Indication; the hostname sent in the ClientHello
  that in-path filters can inspect.
- **RST** — TCP reset, an active rejection rather than a silent drop.
- **Blackhole** — traffic that is silently discarded with no ICMP error and no
  response.
- **DoH** — DNS over HTTPS, used as a comparison resolver path.
- **Single vantage point** — the limitation that one machine cannot distinguish
  local policy, ISP policy and national filtering on its own.

# Assumptions Ledger

| Assumption / decision                                                                                                                                    | Source                      | Resolved |
| :------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------- | :------- |
| Preset catalog is curated global + RU-relevant: YouTube, Telegram, Discord, GitHub, Google, Cloudflare, Wikipedia, X, Tor, plus 1.1.1.1 and 8.8.8.8      | [CL-1 Q1]                   | yes      |
| Per-target ladder is the full diagnostic set                                                                                                             | [CL-1 Q2]                   | yes      |
| Traceroute is UDP tracepath-style by default; ICMP when elevated                                                                                         | [CL-1 Q3]                   | yes      |
| Classification uses the full taxonomy with confidence and evidence                                                                                       | [CL-1 Q4]                   | yes      |
| Custom targets accept hostname or IPv4/IPv6 with optional port and protocol                                                                              | [CL-1 Q5]                   | yes      |
| Probes run sequentially per target, up to 4 targets in parallel                                                                                          | [CL-1 Q6]                   | yes      |
| Results are live TUI cards plus on-demand JSON export; no database                                                                                       | [CL-1 Q7]                   | yes      |
| No external enrichment in v1                                                                                                                             | [CL-1 Q8]                   | yes      |
| Continuous MTR, TCP SYN tracing, distributed measurement, ASN/BGP, throttling quantification and Windows remain out of v1                                | [CL-2 Q1]                   | yes      |
| Proxy/VPN state is shown as context only; no direct-versus-proxied A/B                                                                                   | [CL-2 Q2]                   | yes      |
| Multiple targets may be selected per run (implied by the 4-parallel scheduling choice)                                                                   | analyst, [CL-1 Q6]          | yes      |
| Preset catalog is static and built into the binary in v1; no remote updates                                                                              | [CL-1 Q1], analyst          | yes      |
| Default ports, timeouts, retries and confidence thresholds are technical defaults set in the technical spec                                              | analyst                     | yes      |
| Throttling is inferred only from coarse timing and loss observations in v1; no throughput test exists, so a throttling verdict is low confidence at best | [CL-1 Q4], [CL-2 Q1]        | yes      |
| Session results live in memory and are exported on demand; nothing persists after exit unless exported                                                   | [CL-1 Q7]                   | yes      |
| UI is in English; verdicts include the single-vantage-point caveat                                                                                       | registration batch, analyst | yes      |
| Target picker is multi-select with checkboxes, `space` toggles, `/` filters, inline custom target entry and `Enter` to run                               | [CL-3 Q2]                   | yes      |
| Run view uses target tabs with Ladder/Hops/Verdict inner views; wide terminals show ladder and hops side by side, narrow terminals switch between them   | [CL-3 Q3]                   | yes      |
| Keyboard map: arrows and j/k navigate, Enter drills in, Esc goes back, single-letter actions, `?` opens help                                             | [CL-3 Q4]                   | yes      |
| Visual policy: semantic colors plus symbols and text, 256/16-color fallbacks, `NO_COLOR`, ASCII fallback; status never relies on color alone             | [CL-3 Q5]                   | yes      |
| Terminal sizes: normal from 80x24, compact from 60x20, resize message below that, refuse below 40x10                                                     | [CL-3 Q6]                   | yes      |
| Export: `e` writes a timestamped JSON file and shows a toast; `E` opens a path prompt                                                                    | [CL-3 Q7]                   | yes      |
| Cancellation: `x` cancels current, `X` cancels all, `Ctrl-C` confirms quit and keeps partial results; `Esc` only navigates back                          | [CL-3 Q8]                   | yes      |

# Non-Goals

- Continuous mtr-style monitoring and Paris/Dublin ECMP analysis.
- TCP SYN traceroute with packet capture, or any packet-capture dependency.
- Distributed measurement (RIPE Atlas, Globalping, OONI) or control-server
  comparison in v1.
- ASN, GeoIP or BGP enrichment and looking-glass integration.
- Throughput or throttling quantification beyond what the ladder observes.
- Windows support in v1.
- Automatic remediation, proxy toggling or network reconfiguration.
- Persistent history database or cross-run trend analysis.
- Browser/web UI or mobile app.
- OONI-style result uploads or any telemetry.
- Accepting full URLs with scheme and path as custom targets in v1.
- Direct-versus-proxy/VPN A/B comparison in v1.

# Atomic Business Requirements

### BR-01 — Target picker and selection

- description: From the dashboard the user opens a target picker that lists
  presets grouped by category with checkboxes, offers inline custom-target entry
  and filters with `/`. `Space` toggles a target, `Enter` runs the selected set,
  and the picker shows exactly what will be probed before starting.
- priority: must
- rationale: The vision starts with choosing a preset host/IP or entering a
  custom one; multi-select and search keep the 11-entry catalog usable without
  leaving the keyboard.
- acceptance criteria (business): Presets show a human label and the address
  that will be probed; checkbox state is visible; `/` filters the catalog
  without losing the current selection; the selected count is visible; a custom
  target can be added from the same screen; an empty selection cannot start a
  run; `Enter` starts the run; the user can return to the dashboard without
  running anything.
- traces to: [V §1], [CL-1 Q1], [CL-1 Q5], [CL-1 Q6], [CL-3 Q2]
- depends on: none

### BR-02 — Custom target validation

- description: The custom-target entry accepts a hostname or an IPv4/IPv6
  literal, an optional port and an optional protocol override, validates the
  input inline, resolves hostnames before probing and shows the resolved
  addresses.
- priority: must
- rationale: The vision allows the user to enter their own host or IP; invalid
  or ambiguous input must not produce misleading diagnostics.
- acceptance criteria (business): A valid target starts a run; an invalid value
  shows a concrete error and does not crash or start a run; IPv6 literals are
  accepted; ports outside the valid range are rejected; resolved IPs are visible
  before or during the run; duplicate selections are de-duplicated.
- traces to: [V §1], [CL-1 Q5]
- depends on: BR-01

### BR-03 — Layered probe ladder

- description: For each target the utility runs DNS resolution (system resolver
  versus DoH, and authoritative where possible), TCP connect on multiple ports,
  a TLS ClientHello with real, decoy and absent SNI, an HTTP GET, and ICMP echo;
  traceroute runs alongside the ladder. Each step is independent and a failure
  does not abort the remaining steps.
- priority: must
- rationale: The vision asks for ping, traceroute and other diagnostics to find
  the failing step; a layered ladder is what distinguishes DNS, transport, TLS
  and HTTP blockage.
- acceptance criteria (business): Every ladder step produces a result or a
  skipped reason; a DNS failure does not prevent TCP probes when an IP is known;
  a failed step does not cancel later steps; all step results are retained for
  the verdict and the export.
- traces to: [V §1], [CL-1 Q2], [PCR-0001 CL-2 Q1]
- depends on: BR-01, BR-02

### BR-04 — Drop-hop localization

- description: The utility determines and displays the path per target,
  including the last responding hop, the first non-responding hop, per-hop loss
  and latency, and maps failing probes to a path segment; when stars are
  ambiguous it reports an inconclusive segment and alternative hypotheses
  instead of a definite drop point.
- priority: must
- rationale: The core of the vision is understanding "at which step packets
  drop".
- acceptance criteria (business): A hop table is shown with per-hop statistics;
  the result states either "drop after hop N" or "inconclusive between hops N
  and M"; the localization is cross-checked against TCP/DNS probe outcomes; a
  non-responding hop is never presented as proof of blocking on its own.
- traces to: [V §1], [CL-1 Q3]
- depends on: BR-03

### BR-05 — Blockage-type classification

- description: The utility produces a ranked hypothesis from the taxonomy — DNS
  tampering, TCP RST injection, IP drop/blackhole, port block, SNI/TLS
  filtering, HTTP block/captive portal, throttling, unknown — with a confidence
  level, the supporting evidence lines and the alternatives considered.
- priority: must
- rationale: The vision asks to determine the blockage type where possible; the
  customer chose the full taxonomy with confidence.
- acceptance criteria (business): Every target ends with a type (or unknown), a
  confidence level and at least one evidence line; a type is never shown without
  evidence; rejected alternatives are visible on demand; the
  single-vantage-point caveat accompanies the verdict; "unknown" is an
  acceptable outcome.
- traces to: [V §1], [CL-1 Q4]
- depends on: BR-03, BR-04

### BR-06 — Probe interpretation integrity

- description: The utility distinguishes a timeout (absence of a response) from
  an active rejection (TCP RST, ICMP unreachable, TLS alert, HTTP error status),
  does not treat timeouts alone as proof of blocking, and accounts for ICMP rate
  limiting and CDN/geo-DNS answer variability when comparing DNS results.
- priority: must
- rationale: Most false positives in blockage diagnostics come from reading
  silence as censorship and from legitimate CDN answer differences.
- acceptance criteria (business): Evidence is labelled active or passive; a
  timeout-only case cannot receive high confidence; a DNS verdict shows both
  answer sets and notes the CDN/geo caveat; ICMP rate limiting is mentioned when
  relevant; RST-based and drop-based verdicts are distinguishable.
- traces to: [V §1], [CL-1 Q2], [CL-1 Q4]
- depends on: BR-03, BR-05

### BR-07 — Preset catalog

- description: The binary ships a static, built-in catalog containing YouTube,
  Telegram, Discord, GitHub, Google, Cloudflare, Wikipedia, X, Tor, and the
  neutral anchors 1.1.1.1 and 8.8.8.8; each entry has a display name, host,
  category and default ports/protocols, and the catalog version is reported in
  results.
- priority: must
- rationale: The customer chose a curated global + RU-relevant list; a stable
  built-in catalog makes results comparable across runs and machines.
- acceptance criteria (business): All listed entries are present and selectable;
  the address to be probed is visible; the catalog works with no configuration
  file; JSON output identifies the catalog version; no remote catalog update
  occurs in v1.
- traces to: [V §1], [CL-1 Q1]
- depends on: BR-01

### BR-08 — Scheduling and concurrency

- description: Selected targets run in parallel with a maximum of four
  concurrent targets; probes within a target run sequentially in ladder order;
  per-probe timeouts and retries are bounded. `x` cancels the current target,
  `X` cancels all targets, and `Ctrl-C` asks to quit while keeping partial
  results; `Esc` only navigates back.
- priority: must
- rationale: The customer chose sequential-per-target with four parallel targets
  to keep evidence readable and load bounded.
- acceptance criteria (business): No more than four targets run at once;
  per-target step order is preserved; `x` stops only the current target; `X`
  stops every target; cancellation takes effect promptly and keeps partial
  results exportable; `Ctrl-C` prompts before quitting; `Esc` never cancels a
  run; the UI stays responsive during a run; a hung probe cannot stall the whole
  session beyond its timeout.
- traces to: [V §1], [CL-1 Q6], [CL-3 Q8]
- depends on: BR-03

### BR-09 — Live results UI

- description: The run view shows target tabs on top and Ladder, Hops and
  Verdict as inner views for the active target. Wide terminals show the ladder
  and hop table side by side; narrow terminals switch between them. The view
  shows the current step, evidence, verdict and confidence, and uses color plus
  a symbol or text so status remains readable without color. The user can
  navigate targets, steps and hops, re-run, cancel, export and open help using
  the keyboard.
- priority: must
- rationale: The vision requires terminal display and keyboard control; tabs and
  adaptive panes keep the active target readable while the ladder runs.
- acceptance criteria (business): Progress is visible for the running target and
  step; target tabs show running, done and cancelled states; every target has a
  completed view at the end; wide terminals show ladder and hops together while
  narrow terminals switch between them; status is readable in monochrome and
  ASCII fallback; `NO_COLOR` is honored; normal layout works from 80x24, compact
  from 60x20, a resize message appears below 60x20 and rendering stops below
  40x10; all actions are reachable by keyboard; help is reachable from the
  results view; resizing reflows without losing state; quitting mid-run is
  possible.
- traces to: [V §1], [V §3], [CL-1 Q6], [CL-1 Q7], [CL-3 Q1], [CL-3 Q3], [CL-3
  Q4], [CL-3 Q5], [CL-3 Q6]
- depends on: BR-05, BR-08

### BR-10 — JSON session export

- description: The user can export the full session — targets, ladder steps, raw
  evidence, hops, verdict, confidence, timestamps, privilege mode and catalog
  version — as JSON on demand, with no database or history retained. `e` writes
  a timestamped `egress-<timestamp>.json` to the current directory and shows the
  path in a toast; `E` opens a path prompt.
- priority: must
- rationale: The customer chose JSON export so findings can be attached to
  reports and compared between networks.
- acceptance criteria (business): The export validates against a documented
  schema; it contains the same evidence as the UI; the default filename is
  timestamped; the path is shown after export; existing files are not
  overwritten without confirmation; export works after a completed run and for
  partial results after cancellation; no secrets or unrelated host data are
  included.
- traces to: [V §1], [CL-1 Q7], [CL-3 Q7]
- depends on: BR-05, BR-09

### BR-11 — Privilege handling

- description: Unprivileged mode runs the complete ladder using UDP
  tracepath-style tracing; elevated mode adds ICMP traceroute; the UI marks the
  session as limited or elevated and explains what is unavailable.
- priority: must
- rationale: The customer chose unprivileged-by-default with optional elevation,
  and the ladder must remain useful without root.
- acceptance criteria (business): An unprivileged run completes the full ladder
  with UDP tracing; elevation adds ICMP tracing without changing the meaning of
  other results; the mode is visible in the UI and JSON; the tool never prompts
  for or attempts privilege escalation.
- traces to: [V §1], [CL-1 Q3], [PCR-0001 CL-1 Q4]
- depends on: BR-03

### BR-12 — Local-only operation and probe transparency

- description: The utility performs no telemetry, uploads or enrichment calls;
  the only outbound traffic is probes to selected targets and declared public
  resolvers; the destinations used for a run are visible in the UI and the
  export.
- priority: must
- rationale: The customer required local-only operation and allowed public DoH
  resolvers only as part of the measurement.
- acceptance criteria (business): The run's destination list is viewable; no
  requests go anywhere else; no persistent identifiers are created; behavior
  without internet is explicit; the privacy rule is documented.
- traces to: [CL-1 Q8], [PCR-0001 CL-1 Q7], [PCR-0001 CL-2 Q1]
- depends on: BR-03

### BR-13 — Proxy and VPN context

- description: Each run shows detected proxy environment settings and VPN-like
  interfaces or routes as context for the verdict; the utility does not bypass
  them or run direct-versus-proxied comparisons.
- priority: should
- rationale: The customer chose context only, because proxy/VPN state changes
  what a probe measures.
- acceptance criteria (business): Detected settings and their source are shown;
  absence is shown explicitly; no bypass or A/B run occurs; the context appears
  in the UI and JSON.
- traces to: [CL-2 Q2], [PCR-0001 CL-1 Q1]
- depends on: BR-09

### BR-14 — Limitations disclosure

- description: Every verdict, results view and export states the
  single-vantage-point limitation, the main false-positive risks of the probes
  used, and the fact that absence of evidence is not proof of blocking; it
  suggests re-running from another network as external validation.
- priority: should
- rationale: The tool must not overclaim; the research shows one vantage point
  cannot separate local, ISP and national filtering.
- acceptance criteria (business): The caveat is present in the UI and JSON for
  every target; "unknown" is preferred when evidence is insufficient; the
  suggested external validation is visible without leaving the tool.
- traces to: [V §1], [CL-1 Q4]
- depends on: BR-05, BR-06

### BR-15 — Tool survey and feature rationale

- description: The repository contains a short document comparing existing
  diagnostic tools (ping, traceroute, mtr, tcptraceroute, nexttrace, trippy,
  dig/dog, curl, OONI Probe, RIPE Atlas) and listing which features were
  adopted, deferred or rejected for v1, with links to the corresponding BRs.
- priority: should
- rationale: The vision explicitly asks to review existing solutions and select
  the needed functionality rather than reinvent it.
- acceptance criteria (business): The document names each surveyed tool, its
  relevant capability and its privilege model; adopted and excluded features are
  listed with reasons; every adopted feature maps to a BR; the document is
  updated when scope changes.
- traces to: [V §2]
- depends on: none

### BR-16 — In-app help

- description: An in-app help overlay explains target selection, ladder steps,
  hop-table reading, the blockage taxonomy, confidence levels and keyboard
  shortcuts.
- priority: could
- rationale: The classification model is only useful if the user can understand
  what each verdict means.
- acceptance criteria (business): Help is reachable from the picker and results
  views; it covers all keys and taxonomy terms; it can be dismissed without
  losing the run.
- traces to: [V §1], [V §3], [CL-1 Q4]
- depends on: BR-09
