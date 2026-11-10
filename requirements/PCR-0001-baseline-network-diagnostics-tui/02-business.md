---
pcr: PCR-0001
title: Baseline network diagnostics and TUI shell
status: proposed
revised: 2026-11-10
based_on: 01-vision.md
approved_by: null
approved_at: null
revisions: []
---

# Context & Goals

PCR-0001 defines the foundation of a Go terminal utility for diagnosing network
blockages on macOS and Linux. At launch it automatically runs a baseline health
check of the local network path and presents a keyboard-driven dashboard with
per-check evidence and an overall verdict. The dashboard is also the entry point
to the interactive target diagnostics specified in PCR-0002. The tool is
diagnostic only: it observes and reports, never repairs or reconfigures the
host.

# Stakeholders

- **Terminal user** — needs a fast, clear answer to "is my network broken or is
  something blocking me?", with evidence rather than a bare verdict.
- **Support / helpdesk** — receives exported JSON reports and needs
  reproducible, machine-readable evidence.
- **Maintainer** — needs a bounded v1 scope, predictable platform support and no
  external service dependencies.
- **Privacy-sensitive user** — needs assurance that diagnostics do not become
  telemetry or uploads.

# Glossary

- **Baseline** — the automatic set of checks the utility runs on launch to
  assess the local network path.
- **Check** — one baseline probe with a target, method, status, evidence and
  timestamp.
- **Anchor** — a neutral external endpoint used as a reference point (e.g.
  Cloudflare 1.1.1.1, Google 8.8.8.8).
- **Verdict** — the aggregate health summary derived from check statuses.
- **Evidence** — the measured values (latency, loss, response code, error,
  resolver answer) that justify a check status.
- **TUI** — full-screen terminal user interface driven by the keyboard.
- **Unprivileged mode** — running as the invoking user without raw sockets or
  elevated capabilities (the default).
- **Elevated mode** — running with sufficient privileges to enable raw-socket
  checks such as ICMP traceroute.
- **DoH** — DNS over HTTPS; a public resolver is used as a comparison path
  against the system resolver.
- **Captive portal** — an intercepting HTTP page (hotel, airport, corporate
  network) that replaces the expected response.
- **PMTU** — path maximum transmission unit; a blackhole occurs when large
  packets are dropped while small ones pass.

# Assumptions Ledger

| Assumption / decision                                                                                                                        | Source               | Resolved |
| :------------------------------------------------------------------------------------------------------------------------------------------- | :------------------- | :------- |
| Artifacts and UI are in English (repo language is English)                                                                                   | registration batch   | yes      |
| v1 targets macOS and Linux                                                                                                                   | [CL-1 Q5]            | yes      |
| Baseline completes within 30 seconds on a typical network                                                                                    | [CL-1 Q3]            | yes      |
| Baseline covers the full recommended check set                                                                                               | [CL-1 Q1]            | yes      |
| Baseline auto-runs at launch and lands on a dashboard                                                                                        | [CL-1 Q2]            | yes      |
| Unprivileged by default; elevation is optional and explicit                                                                                  | [CL-1 Q4]            | yes      |
| TUI is primary; `--json` and non-interactive modes exist                                                                                     | [CL-1 Q6]            | yes      |
| No telemetry, uploads or enrichment APIs; diagnostic probes to public resolvers/anchors are allowed                                          | [CL-1 Q7], [CL-2 Q1] | yes      |
| Curated multi-vendor anchor set is used                                                                                                      | [CL-1 Q8]            | yes      |
| Status model is pass / warn / fail / skipped, always backed by evidence                                                                      | analyst decision     | yes      |
| No automatic remediation or host reconfiguration                                                                                             | [V §1], analyst      | yes      |
| Per-probe timeouts, retries and thresholds are technical defaults set in the technical spec                                                  | analyst              | yes      |
| Only one baseline run executes at a time; re-run replaces the displayed result                                                               | analyst              | yes      |
| Non-TTY execution falls back to non-interactive output                                                                                       | [CL-1 Q6]            | yes      |
| A single vantage point cannot prove nationwide or ISP-wide blocking; the tool reports evidence and confidence, not certainty                 | research, analyst    | yes      |
| Baseline history is not persisted; JSON export is the only artifact                                                                          | [CL-1 Q6]            | yes      |
| TUI uses master-detail: path list on the left, selected check evidence on the right; narrow terminals collapse to a stacked view             | [CL-3 Q1]            | yes      |
| Keyboard map: arrows and j/k navigate, Enter drills in, Esc goes back, single-letter actions, `?` opens help                                 | [CL-3 Q4]            | yes      |
| Visual policy: semantic colors plus symbols and text, 256/16-color fallbacks, `NO_COLOR`, ASCII fallback; status never relies on color alone | [CL-3 Q5]            | yes      |
| Terminal sizes: normal from 80x24, compact from 60x20, resize message below that, refuse below 40x10                                         | [CL-3 Q6]            | yes      |
| Export: `e` writes a timestamped JSON file and shows a toast; `E` opens a path prompt                                                        | [CL-3 Q7]            | yes      |
| Cancellation: `x` cancels current, `X` cancels all, `Ctrl-C` confirms quit and keeps partial results; `Esc` only navigates back              | [CL-3 Q8]            | yes      |

# Non-Goals

- Automatic repair, reconfiguration, proxy toggling or "fix" actions.
- Windows support in v1.
- Telemetry, analytics, crash reporting, result uploads or third-party
  enrichment APIs.
- Throughput or bandwidth benchmarking.
- Packet capture, deep packet inspection or traffic interception.
- Persistent history database or cross-run analytics.
- Distributed measurement (RIPE Atlas, Globalping, OONI) in v1.
- A general-purpose replacement for ping, traceroute or mtr.
- A GUI, web UI or mobile app.
- User-configurable replacement of the anchor set in v1.

# Atomic Business Requirements

### BR-01 — Automatic baseline on launch

- description: When started in an interactive terminal, the utility runs the
  baseline automatically, shows live progress, and lands on the dashboard
  without requiring input or privilege escalation.
- priority: must
- rationale: The vision starts with "launch the utility, it runs baseline
  diagnostics and shows results"; any extra step contradicts it.
- acceptance criteria (business): Launching in a TTY begins the baseline with no
  keypress; checks taking longer than one second show progress; the dashboard is
  reachable within the 30-second budget; no sudo prompt appears; launching
  without internet still reaches a dashboard with a degraded result.
- traces to: [V §2], [CL-1 Q2], [CL-1 Q3]
- depends on: none

### BR-02 — Baseline coverage

- description: The baseline includes interface and default-route inspection,
  gateway reachability, DNS configuration and canary resolution, proxy
  environment detection, system resolver versus DoH comparison, ICMP to gateway
  and anchors, TCP 443/53 to anchors, a captive-portal HTTP canary, an MTU/PMTU
  probe, and a latency/jitter/loss sample.
- priority: must
- rationale: The customer explicitly chose the full recommended set because a
  partial baseline cannot separate local, DNS, transport and HTTP-layer
  failures.
- acceptance criteria (business): Every listed check either produces a result or
  is marked skipped with a stated reason; no check disappears silently; the
  check list is documented and identical in the TUI and JSON output.
- traces to: [V §2], [CL-1 Q1], [CL-2 Q1]
- depends on: BR-01

### BR-03 — Evidence-bearing check results

- description: Every check result carries a status (pass, warn, fail, skipped),
  its target, method, measured values, timestamp, and error or diagnostic text
  where applicable; a status is never shown without supporting evidence.
- priority: must
- rationale: A bare "blocked" verdict is not actionable and cannot be trusted;
  the tool must show why it reached a conclusion.
- acceptance criteria (business): The dashboard and JSON expose the same
  evidence fields; every fail or warn includes at least one measured value or
  error; every skipped check states the missing capability or condition;
  timestamps allow ordering of events.
- traces to: [V §2], [CL-1 Q1]
- depends on: BR-02

### BR-04 — Dashboard and overall verdict

- description: The dashboard uses a master-detail layout: checks are grouped by
  path layer (local link → gateway → DNS → transport → HTTP) in a list on the
  left, and the selected check's evidence appears on the right; narrow terminals
  collapse to a stacked view. It shows an overall verdict, status counts,
  elapsed time and the most actionable finding, and derives the verdict
  deterministically from check statuses.
- priority: must
- rationale: The vision asks for displayed results; grouping by layer and
  showing evidence next to the list makes a blockage understandable at a glance.
- acceptance criteria (business): The overall verdict follows a documented
  aggregation rule; re-running updates the dashboard in place; the most severe
  finding is identifiable without opening every check; no overall verdict is
  produced when no check has run; at 80x24 or wider the list and evidence pane
  are both visible; below 80x24 the layout collapses without losing the selected
  check; status uses color plus a symbol or text and stays readable in
  monochrome; resizing reflows without losing state.
- traces to: [V §2], [V §3], [CL-1 Q2], [CL-3 Q1], [CL-3 Q5], [CL-3 Q6]
- depends on: BR-03

### BR-05 — Keyboard-only operation

- description: Every function — navigating checks, opening details, re-running,
  entering target mode, opening help and quitting — is reachable by keyboard
  using arrows and `j`/`k` to move, `Enter` to drill in, `Esc` to go back,
  single-letter shortcuts for actions and `?` for help. A visible key-hint bar
  and an in-app help overlay expose the map; the mouse is optional.
- priority: must
- rationale: The vision explicitly requires keyboard control and terminal-only
  display.
- acceptance criteria (business): A user can review the whole baseline without a
  mouse; the footer shows contextual key hints; every key is documented in the
  help overlay; `Esc` only navigates back; `Ctrl-C` asks for confirmation and
  exits cleanly; resizing the terminal keeps content usable.
- traces to: [V §3], [CL-3 Q4], [CL-3 Q8]
- depends on: BR-04

### BR-06 — Unprivileged by default

- description: The utility performs its core checks as the invoking user without
  requesting or requiring elevated privileges; checks that need raw sockets are
  marked limited or skipped with an explanation and an elevation hint.
- priority: must
- rationale: The customer chose unprivileged-by-default with optional elevation
  to keep startup safe and frictionless.
- acceptance criteria (business): No privilege prompt appears; one unprivileged
  check cannot fail the whole baseline; each limited check names the missing
  capability and how to enable it; the tool never re-execs itself with sudo.
- traces to: [CL-1 Q4]
- depends on: BR-02

### BR-07 — Time budget and bounded probes

- description: The baseline targets completion within 30 seconds on a typical
  network using per-probe timeouts, bounded retries and parallel independent
  probes; a probe that exceeds its budget is reported as timed out instead of
  hanging the run. The user can cancel the baseline with `x`, `X` or `Ctrl-C`;
  cancellation stops new probes, retains completed checks and reports a
  cancelled result.
- priority: must
- rationale: The customer accepted a 30-second budget; an unbounded diagnostic
  tool is worse than no tool on a broken network.
- acceptance criteria (business): A typical run completes within 30 seconds; the
  dashboard remains reachable after per-probe budgets expire; timed-out checks
  have a status and evidence; cancellation takes effect promptly; completed
  checks remain visible; a cancelled run is labelled and exportable; the user
  can quit immediately at any point.
- traces to: [V §2], [CL-1 Q3], [CL-3 Q8]
- depends on: BR-01, BR-02

### BR-08 — macOS and Linux support

- description: v1 builds and runs on macOS and Linux, using OS-appropriate
  mechanisms for interfaces, routes, DNS configuration and ICMP, and surfaces
  platform-specific differences in the evidence instead of hiding them.
- priority: must
- rationale: The customer chose macOS and Linux as the v1 platform set.
- acceptance criteria (business): The tool starts and completes a baseline on
  both platforms; unsupported platform mechanisms produce an explicit skipped or
  limited result; no platform-specific crash when a mechanism is absent.
- traces to: [CL-1 Q5]
- depends on: BR-02

### BR-09 — Local-only privacy

- description: The utility sends no telemetry, analytics, crash reports or
  result uploads and calls no enrichment APIs; the only outbound traffic is the
  documented diagnostic probes to configured anchors/targets, including public
  DoH resolvers.
- priority: must
- rationale: The customer required local-only operation and explicitly allowed
  diagnostic probes as the measurement itself.
- acceptance criteria (business): No network traffic exists beyond documented
  probes; no persistent identifiers are created or stored; the destinations are
  listed in the UI or documentation; the tool remains useful with no internet,
  reporting local diagnostics; no hidden background calls occur.
- traces to: [CL-1 Q7], [CL-2 Q1]
- depends on: BR-02

### BR-10 — Curated multi-vendor anchors

- description: The baseline uses a curated anchor set — Cloudflare 1.1.1.1,
  Google 8.8.8.8, the Apple captive-portal canary and example.com — reports each
  anchor individually, and never lets one vendor's failure alone determine the
  overall verdict.
- priority: must
- rationale: Multi-vendor anchors distinguish a single-service problem from a
  path-wide problem.
- acceptance criteria (business): Each anchor result is visible; one failing
  anchor while others pass yields a warning rather than a global "network
  blocked"; the anchor list is documented and used consistently in TUI and JSON.
- traces to: [CL-1 Q8]
- depends on: BR-02

### BR-11 — Offline and degraded handling

- description: When connectivity is absent or degraded, the utility
  distinguishes local link/interface problems, gateway unreachability, DNS-only
  failure and upstream failure, reports what could be tested locally, and does
  not mislabel missing connectivity as censorship.
- priority: must
- rationale: "No internet" and "blocked service" require different user actions;
  conflating them would make the tool misleading.
- acceptance criteria (business): A no-route or unplugged run produces a local
  diagnosis; the baseline still completes; wording avoids blockage claims
  without evidence; re-running after recovery produces a fresh result.
- traces to: [V §2], [CL-1 Q1], [CL-1 Q3]
- depends on: BR-02, BR-04

### BR-12 — Captive portal and PMTU checks

- description: The baseline detects captive portals by comparing an HTTP canary
  response against the expected status/body and probes path MTU with the
  don't-fragment flag; portal detection reports the portal and never
  auto-authenticates or changes settings.
- priority: must
- rationale: Captive portals and PMTU blackholes mimic blockage symptoms and are
  in the selected baseline scope.
- acceptance criteria (business): A portal produces a distinct warning showing
  the observed redirect or body; PMTU blackhole suspicion is reported with the
  probe sizes used; no host configuration is changed.
- traces to: [CL-1 Q1]
- depends on: BR-02

### BR-13 — Optional elevated mode

- description: When started with sufficient privileges, the utility enables
  raw-socket-dependent checks and clearly marks the session as elevated; results
  remain comparable with unprivileged runs.
- priority: should
- rationale: Elevation unlocks richer path evidence without being required for
  basic diagnostics.
- acceptance criteria (business): An elevated session exposes additional checks;
  the UI and JSON label the mode; no check silently changes meaning between
  modes; the tool does not escalate privileges on its own.
- traces to: [CL-1 Q4]
- depends on: BR-06

### BR-14 — JSON and non-interactive output

- description: The utility offers `--json` and a non-interactive mode that emit
  the complete baseline result — checks, evidence, verdict, timestamps, mode and
  platform — as stable machine-readable JSON on stdout, with documented exit
  codes reflecting the verdict. In the interactive TUI, `e` writes a timestamped
  JSON file to the current directory and shows the path in a toast; `E` opens a
  path prompt.
- priority: should
- rationale: The customer chose scriptable output so results can be attached to
  bug reports and compared later.
- acceptance criteria (business): Output validates against a documented schema;
  no ANSI or TUI escapes appear when stdout is not a TTY; exit codes distinguish
  healthy, degraded and failed outcomes; JSON contains the same evidence as the
  TUI; the default filename is timestamped; the path is shown after writing;
  existing files are not overwritten without confirmation; export works for
  completed and cancelled runs.
- traces to: [CL-1 Q6], [CL-3 Q7], [V §3]
- depends on: BR-03, BR-04

### BR-15 — Proxy and VPN context

- description: The baseline detects proxy environment variables and VPN-like
  interfaces or routes and displays them as context for interpreting results; it
  does not bypass them or perform direct-versus-proxied comparisons.
- priority: should
- rationale: Proxy/VPN state changes what a probe means; showing it prevents
  misreading a result. The customer chose context only for v1.
- acceptance criteria (business): Detected settings are shown with their source;
  no proxy bypass or A/B run occurs; absence of proxy/VPN is also shown
  explicitly.
- traces to: [CL-1 Q1], [PCR-0002 CL-2 Q2]
- depends on: BR-02

### BR-16 — Help and documentation

- description: An in-app help overlay lists keys, baseline checks and verdict
  semantics, and the repository README explains installation, unprivileged
  versus elevated mode, privacy behavior and JSON usage.
- priority: could
- rationale: A terminal tool without discoverable keys or documented semantics
  is hard to adopt and easy to misread.
- acceptance criteria (business): Keys are discoverable without reading the
  README; the README covers both modes and the privacy rule; help is reachable
  from the dashboard.
- traces to: [V §3], [CL-1 Q4], [CL-1 Q6]
- depends on: BR-04
