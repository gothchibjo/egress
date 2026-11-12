---
pcr: PCR-0001
title: Baseline network diagnostics and TUI shell
status: implementing
basis: 03-technical.yaml (frozen)
revised: 2026-11-12
approved_by: gothchibjo
approved_at: 2026-11-11
---

# Plan Invariants

- The contract is sealed: `03-technical.yaml` is `frozen` and its REQ titles,
  descriptions, acceptance criteria, dependencies, verification and guard
  conditions are not editable in this plan.
- Every task below implements an existing REQ. A missing or contradictory
  requirement is not patched here; it routes to change control as a new PCR via
  `req-workflow`.
- The plan changes implementation structure only. It never changes the meaning
  of a check, verdict, status or output field.
- Each REQ is worked only after `req-guards` passes its guard conditions, and is
  closed only with the evidence named in its section.
- The binary is named `egress`. Export files use `egress-<timestamp>.json`.
- Development is Go on macOS and Linux. The module path is
  `github.com/gothchibjo/egress` from the repository remote; `go.mod` pins the
  installed Go toolchain.
- Domain types stay independent of the TUI, OS probing and I/O; TUI, probe and
  report adapters depend on the domain, never the reverse.

# Execution Order

Order is topological by frozen `dependencies`, ties by REQ id. The package map
is a plan decision: `cmd/egress`, `internal/domain`, `internal/app`,
`internal/probe`, `internal/platform`, `internal/report`, `internal/tui`.
External dependencies are limited to Bubble Tea and Lip Gloss for the terminal
UI, `github.com/miekg/dns` for DNS wire queries, and `golang.org/x/net` for
ICMP. Any additional dependency needs a stated reason in the REQ section.

## REQ-001 — Check result and evidence model (must, s)

- Create `internal/domain` with `Status` (`pass`, `warn`, `fail`, `skipped`),
  `CheckID`, `Result` (check id, status, target, method, measured values,
  timestamp, diagnostic, error) and validating constructors.
- Reject construction of `warn`/`fail` without a measured value or error, and of
  `skipped` without a stated reason.
- Define stable JSON field names and one serializer shared by the TUI and the
  report adapter; the domain imports neither.
- Add table-driven unit tests for status invariants and JSON round-trip.
- Guards: BR-03 remains approved; no result presents a status without evidence.
- Evidence: unit test report and JSON schema validation report (method: test).

## REQ-002 — Baseline orchestration and progress (must, m)

- Create `internal/app` with a single-flight baseline runner that starts on TTY
  launch without a keypress, emits progress for checks longer than one second,
  and hands a completed run to the dashboard.
- Replace the previous run on re-run; never merge runs.
- Keep startup free of privilege prompts and make the offline path reach the
  dashboard with a degraded verdict.
- Add tests with fake checks and a fake clock for single-flight, progress and
  replacement behaviour.
- Guards: no elevation request on startup; a re-run replaces the previous run.
- Evidence: unit test report and TUI walkthrough notes (method: test).

## REQ-003 — Curated multi-vendor anchor set (must, xs)

- Define the anchor set as domain data: Cloudflare `1.1.1.1`, Google `8.8.8.8`,
  the Apple captive-portal canary `captive.apple.com` and `example.com`.
- Give each anchor a stable id, display name, vendor and purpose; document the
  list once and reuse it in TUI and JSON.
- Add a test asserting all four anchors are present in every baseline plan and
  that one failing anchor cannot alone produce a blocked verdict.
- Guards: the set stays multi-vendor; no single vendor determines the verdict.
- Evidence: anchor list review sign-off and traceability matrix (method:
  inspection).

## REQ-004 — Local link and default route inspection (must, s)

- Define a platform adapter for interfaces, link state, addresses and the
  default route in `internal/platform`, with `darwin` and `linux`
  implementations.
- Linux: read interface data from the standard library and the default route
  from `/proc/net/route`. macOS: read interface data from the standard library
  and the default route from `route -n get default`.
- Report each interface, its link state and addresses; report a missing default
  route as a distinct local failure with evidence.
- Keep inspection read-only; add parser fixture tests plus a platform test run
  on both OSes.
- Guards: inspection never changes interface or route configuration; macOS and
  Linux only.
- Evidence: platform test report and traceability matrix (method: test).

## REQ-005 — Gateway reachability (must, s)

- Derive the gateway from the route adapter and probe it with the ICMP adapter
  from REQ-009, reporting round-trip latency or the failure reason.
- Distinguish reachable, unreachable and skipped outcomes; a missing gateway is
  `skipped` with a reason.
- Add tests with a fake ICMP transport and a platform test on macOS and Linux.
- Guards: no network configuration is changed.
- Evidence: unit test report and platform test report (method: test).

## REQ-006 — DNS configuration and canary resolution (must, s)

- Add a resolver-configuration adapter: `/etc/resolv.conf` on Linux and
  `scutil --dns` on macOS; parse nameserver addresses without changing them.
- Resolve the canary hostname with `github.com/miekg/dns` against each
  configured resolver, reporting answers, response code and latency.
- Distinguish `NXDOMAIN`, `SERVFAIL` and timeout; a missing resolver
  configuration is `fail` with a reason.
- Add fixture tests for both configuration formats and an opt-in local DNS
  integration test.
- Guards: no DNS configuration is changed.
- Evidence: unit test report and integration test report (method: test).

## REQ-007 — System resolver versus DoH comparison (must, m)

- Add a DoH client over HTTPS using the documented public resolvers, exchanging
  DNS messages with `miekg/dns`; allow no other endpoints.
- Compare the system-resolver answer with the DoH answer, report both answer
  sets and the comparison outcome.
- Flag differing sets with the CDN and geo-DNS caveat; a DoH failure is a
  separate result and never erases the system answer.
- Add unit tests with an `httptest` DoH endpoint and an opt-in live integration
  test.
- Guards: public DoH resolvers are diagnostic probes only; no enrichment API is
  called.
- Evidence: unit test report and integration test report (method: test).

## REQ-008 — Proxy environment detection (must, xs)

- Read `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, `NO_PROXY` and their lowercase
  variants, recording value source per setting.
- Report detected settings with source, or state explicitly that none is set.
- Keep the detected proxy in effect for baseline HTTP probes; no bypass and no
  direct-versus-proxied run.
- Add unit tests using `t.Setenv` for set, unset and mixed-case combinations.
- Guards: no proxy bypass or direct-versus-proxied comparison in v1.
- Evidence: unit test report and traceability matrix (method: test).

## REQ-009 — ICMP reachability to gateway and anchors (must, m)

- Add an ICMP adapter using `golang.org/x/net/icmp`, preferring unprivileged
  datagram sockets (`udp4`/`udp6`) and using raw sockets only in elevated mode.
- Send a bounded echo sample to the gateway and each anchor; report
  reachability, round-trip time and loss per destination.
- Name ICMP rate limiting as a caveat when responses are absent; when the socket
  kind is unavailable, emit a `limited` result with an elevation hint.
- Add fake-transport unit tests, an opt-in integration test and a platform test.
- Guards: unprivileged mode works without raw sockets; raw sockets only in
  explicitly elevated mode.
- Evidence: integration test report and platform test report (method: test).

## REQ-010 — TCP reachability to anchors (must, s)

- Probe port 443 and port 53 on every anchor with a bounded `net.Dialer`
  timeout; record connect latency or the failure class.
- Distinguish refusal, timeout and reset, and label active rejection versus
  passive timeout in the result.
- Add tests against a local listener and a closed port, plus an opt-in live
  test.
- Guards: probes go only to configured anchors and documented destinations;
  per-probe timeouts are bounded.
- Evidence: integration test report and traceability matrix (method: test).

## REQ-011 — Captive portal HTTP canary (must, m)

- Request the Apple captive-portal canary over HTTP with redirects captured
  rather than followed, and compare status, headers and body fingerprint against
  the documented expected response.
- Report a captive portal as a distinct `warn` result with the observed redirect
  or body; never authenticate or change settings.
- Make exactly one HTTP request for this check.
- Add a simulated portal test with `httptest` for the redirect, altered-body and
  healthy cases.
- Guards: no authentication or configuration change; one request per check.
- Evidence: simulated portal test report and TUI walkthrough notes (method:
  simulation).

## REQ-012 — MTU and PMTU probe (must, m)

- Implement a raw-socket DF probe in elevated mode over the documented packet
  size range, reporting every size tested and any failing size.
- Report a suspected PMTU blackhole with the failing size; skip with a reason
  when the path is unreachable or raw sockets are unavailable.
- Never change the interface MTU or any host setting.
- Add a simulated prober test covering blackhole, clean path and skipped cases.
- Guards: the probe is read-only; packet sizes stay within the documented range.
- Evidence: simulated PMTU test report and traceability matrix (method:
  simulation).

## REQ-013 — Latency, jitter and loss sample (must, s)

- Reuse the bounded ICMP sample from REQ-009; do not add probes that extend the
  baseline budget.
- Report sample size, minimum, median, maximum, jitter and loss derived from the
  same sample.
- Add unit tests with known RTT series for odd/even samples, all-loss and
  single-response cases.
- Guards: the sample is bounded and cannot extend the baseline beyond its
  budget.
- Evidence: unit test report and integration test report (method: test).

## REQ-014 — Baseline time budget, timeouts and cancellation (must, m)

- Enforce per-probe timeouts, bounded retries and a 30-second overall budget in
  the runner; report timed-out checks with status and evidence.
- Wire `x` (cancel current), `X` (cancel all) and `Ctrl-C` (confirm quit, keep
  partial results); `Esc` never cancels.
- Retain completed checks and label the run `cancelled`; keep partial results
  exportable and quit reachable at any moment.
- Add timing tests with a fake clock and cancellation tests with blocking fake
  probes.
- Guards: the user can quit immediately at any point; no probe hangs beyond its
  timeout.
- Evidence: timing test report and cancellation test report (method: test).

## REQ-015 — Platform capability abstraction (must, m)

- Define one capability interface covering interfaces, routes, DNS
  configuration, ICMP and tracing; provide `darwin` and `linux` implementations
  behind build tags.
- Map absent mechanisms to `skipped` or `limited` results with the platform
  difference in the evidence; never crash.
- Keep Windows out of the build; record the missing mechanism instead.
- Produce the platform test matrix and a code review sign-off.
- Guards: macOS and Linux are the only v1 platforms; platform behaviour never
  changes a check meaning silently.
- Evidence: platform test matrix and code review sign-off (method: inspection).

## REQ-016 — Unprivileged execution and capability detection (must, m)

- Detect effective privileges and ICMP raw-socket availability at startup
  without prompting.
- Run all non-raw checks as the invoking user; mark raw-socket checks `limited`
  or `skipped` with the missing capability and an elevation hint.
- Ensure one limited check never fails the whole baseline and the process never
  re-execs with `sudo`.
- Add an unprivileged test run and TUI walkthrough.
- Guards: unprivileged is the default; elevation is explicit and user-initiated.
- Evidence: unprivileged test report and TUI walkthrough notes (method: test).

## REQ-017 — Optional elevated mode (should, s)

- When the process already has sufficient privileges, enable the raw-socket
  checks and label the session mode as `elevated`.
- Keep result fields and meanings identical between modes; expose the mode in
  the TUI and JSON.
- Add an elevated test run and compare its result shape with an unprivileged
  run.
- Guards: elevation is explicit; no check changes meaning between modes.
- Evidence: elevated mode test report and traceability matrix (method: test).

## REQ-018 — Aggregate verdict logic (must, m)

- Document the deterministic aggregation rule: per-check status precedence
  (`fail` > `warn` > `skipped` > `pass`), layer scope and the anchor-isolation
  exception.
- Derive the overall verdict, status counts and the most actionable finding from
  the same rule; produce no verdict when no check has run.
- Ensure one failing anchor while others pass yields `warn`, not a global
  blocked verdict.
- Add table-driven tests covering every status combination and the
  anchor-isolation case; record the verdict rule review sign-off.
- Guards: a verdict is never produced without evidence; the rule is
  deterministic for identical results.
- Evidence: unit test report and verdict rule review sign-off (method: test).

## REQ-019 — Offline and degraded classification (must, m)

- Classify a run as local link, gateway, DNS-only or upstream failure using the
  checks already collected; state what could be tested locally.
- Use wording that avoids blockage or censorship claims without evidence.
- Complete the baseline offline and replace the previous result on re-run after
  recovery.
- Add simulated no-route, unplugged and DNS-only scenarios plus a TUI
  walkthrough.
- Guards: missing connectivity is never presented as censorship.
- Evidence: simulated offline test report and TUI walkthrough notes (method:
  simulation).

## REQ-020 — Local-only egress policy (must, s)

- Centralize outbound access behind the probe and DoH clients; add no telemetry,
  analytics, crash reporting, uploads or enrichment clients.
- Expose the documented destination list in the UI and JSON.
- Verify with a network-capture review that only declared probes leave the
  machine, and record a code review sign-off.
- Guards: diagnostic probes are the only outbound traffic; no telemetry or
  enrichment API is added.
- Evidence: network capture review and code review sign-off (method:
  inspection).

## REQ-021 — Keyboard navigation and help overlay (must, m)

- Implement the key map in the TUI: arrows and `j`/`k` move, `Enter` drills in,
  `Esc` goes back, single-letter actions, `?` opens help.
- Show contextual key hints in the footer and document every key in the help
  overlay, including verdict semantics.
- Ensure `Esc` only navigates back and `Ctrl-C` confirms before quitting.
- Walk through the full baseline without a mouse and record key map review
  sign-off.
- Guards: keyboard control is complete; mouse support remains optional.
- Evidence: TUI walkthrough notes and key map review sign-off (method:
  manual-review).

## REQ-022 — JSON and non-interactive output (should, m)

- Add `--json` and a non-interactive mode that emit the complete baseline result
  on stdout as schema-valid JSON with no ANSI or terminal control sequences.
- Publish the JSON schema and document exit codes: `0` healthy, `1` degraded,
  `2` failed, `3` usage or runtime error.
- In the TUI, `e` writes `egress-<timestamp>.json` and shows the path; `E` opens
  a path prompt. Do not overwrite an existing file without confirmation.
- Support export for completed and cancelled runs; add schema validation and CLI
  tests.
- Guards: no secrets or unrelated host data are exported; non-interactive output
  contains no terminal control sequences.
- Evidence: JSON schema validation report and CLI test report (method: test).

## REQ-023 — Proxy and VPN context display (should, s)

- Reuse proxy detection from REQ-008 and add VPN-like interface or route
  detection (`utun`, `tun`, `tap`, `wg` and equivalent routes) as read-only
  context.
- Display detected settings with source, state explicitly when none is found,
  and include the same context in JSON.
- Add unit tests for detection and a TUI walkthrough.
- Guards: context is informational only.
- Evidence: unit test report and TUI walkthrough notes (method: test).

## REQ-024 — README and operational documentation (could, xs)

- Write the repository README: installation, unprivileged and elevated modes,
  local-only privacy rule, JSON output, exit codes and how to discover keys.
- Link the anchor list, verdict rule and JSON schema; keep the text in English.
- Record a documentation review sign-off.
- Guards: documentation is written in English.
- Evidence: documentation review sign-off (method: inspection).

## REQ-025 — Dashboard rendering and adaptive layout (must, l)

- Build the master-detail dashboard with checks grouped by path layer on the
  left and the selected check's evidence on the right.
- Collapse to a stacked view below 80x24 without losing the selected check; keep
  status readable through color plus symbol or text, honor `NO_COLOR`, support
  256-color, 16-color and ASCII fallbacks.
- Implement normal layout from 80x24, compact from 60x20, a resize message below
  60x20 and refuse rendering below 40x10.
- Preserve state across resizes and add a terminal matrix walkthrough.
- Guards: status never depends on color alone; minimum terminal sizes are
  enforced by the UI.
- Evidence: TUI walkthrough notes and terminal matrix (method: manual-review).

# Plan Lifecycle

- A new `plan` event re-drafts this file; the earlier draft moves to `Revisions`
  and is never deleted.
- The `plan` event does not pass `plan_approval`. Implementation starts only
  after explicit human approval, which sets `status: plan_approved`,
  `approved_by`, `approved_at` and `pipeline_gates.plan_approval.passed: true`.
- After approval, `req-guards` pre-checks each REQ; `implement` opens work,
  `verify` records evidence and closes REQs, and `complete` requires evidence
  for every REQ.
- Guard violations set `blocked`; only `resume` leaves that state.
- History in this file is append-only.

# Guard Log

| Date       | REQ     | Guard                                                                        | Check                                                                                                                                                                                                                           | Result |
| :--------- | :------ | :--------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | :----- |
| 2026-11-11 | REQ-001 | BR-03 remains approved; evidence fields cannot be dropped without a new PCR. | `02-business.md` is `approved`; frozen REQ-001 still requires check id, status, target, method, measured values, timestamp and diagnostic/error.                                                                                | pass   |
| 2026-11-11 | REQ-001 | No result may present a status without evidence.                             | No implementation exists yet; the frozen model requires evidence for warn/fail and a stated reason for skipped, and the constructor will enforce both.                                                                          | pass   |
| 2026-11-11 | REQ-002 | The utility never prompts for or requests elevated privileges on startup.    | No startup or elevation code exists; `internal/` contains no sudo, Geteuid or exec call, and the runner performs no privilege operation.                                                                                        | pass   |
| 2026-11-11 | REQ-002 | A previous run is replaced, not merged, when the baseline is re-run.         | The runner holds no history and `Session` stores one current run per completed invocation.                                                                                                                                      | pass   |
| 2026-11-11 | REQ-003 | The anchor set remains multi-vendor.                                         | No anchor data exists yet; the planned set uses Cloudflare, Google, Apple and IANA, one anchor per vendor.                                                                                                                      | pass   |
| 2026-11-11 | REQ-003 | No single vendor determines the overall verdict alone.                       | Verdict logic is not implemented yet; REQ-018 will enforce anchor isolation, and the set has four vendors.                                                                                                                      | pass   |
| 2026-11-12 | REQ-008 | No proxy bypass or direct-versus-proxied comparison occurs in v1.            | No HTTP probe or transport exists yet; the detector only reads and reports environment values and adds no bypass or A/B path.                                                                                                   | pass   |
| 2026-11-12 | REQ-018 | A verdict is never produced without evidence.                                | `AggregateVerdict` returns `ErrNoResults` for an empty run and `ErrInvalidVerdictInput` for any result that fails `Validate`; no level is computed in either case.                                                              | pass   |
| 2026-11-12 | REQ-018 | The rule is deterministic for identical check results.                       | One forward pass, fixed status precedence and baseline-order tie-break; map iteration only accumulates counts and the anchor set.                                                                                               | pass   |
| 2026-11-12 | REQ-025 | Status never depends on color alone.                                         | Every status renders as a symbol plus a status word; the monochrome and ASCII tests assert both and reject ANSI escapes.                                                                                                        | pass   |
| 2026-11-12 | REQ-025 | Minimum terminal sizes are enforced by the UI.                               | `Layout()` stops below 40x10 and shows a resize message below 60x20; `View()` returns an empty string in the stopped state.                                                                                                     | pass   |
| 2026-11-12 | REQ-021 | Keyboard control is complete; mouse support remains optional.                | Every dashboard action has a key in `docs/key-map.md` and `helpLines()`, driven through `tea.KeyMsg` in tests; no mouse handler or mouse mode is registered.                                                                    | pass   |
| 2026-11-12 | REQ-004 | Inspection is read-only and never changes interface or route configuration.  | The adapter calls `net.Interfaces`, `Interface.Addrs`, reads `/proc/net/route` and `/proc/net/ipv6_route`, or runs `route -n get`; no write, sysctl or mutating command exists.                                                 | pass   |
| 2026-11-12 | REQ-004 | macOS and Linux are the only v1 platforms.                                   | Build-tagged `darwin` and `linux` route adapters are the only implementations; every other platform returns `ErrUnsupported` and Windows is never presented as supported.                                                       | pass   |
| 2026-11-12 | REQ-006 | No DNS configuration is changed.                                             | The adapter only reads `/etc/resolv.conf` or runs `scutil --dns`; the check only sends queries through `miekg/dns` and never writes resolver state.                                                                             | pass   |
| 2026-11-12 | REQ-007 | Public DoH resolvers are allowed only as diagnostic probes.                  | The only DoH target is the pinned `DefaultDoHEndpoint`; it is queried once for the canary and never used for general traffic or enrichment.                                                                                     | pass   |
| 2026-11-12 | REQ-007 | No enrichment API is called.                                                 | The DoH JSON request is the only external HTTP call in the DNS check; no other service receives the canary or host data.                                                                                                        | pass   |
| 2026-11-12 | REQ-009 | Unprivileged mode works without raw sockets.                                 | The default dial opens `udp4`/`udp6` ICMP sockets; the live unprivileged run on macOS passes and raw networks appear only in the elevated branch.                                                                               | pass   |
| 2026-11-12 | REQ-009 | Raw sockets are used only in explicitly elevated mode.                       | `dialICMP` selects `ip4:icmp` or `ip6:ipv6-icmp` only when `ICMPOptions.Elevated` is true; the probe never escalates and defaults to datagram sockets.                                                                          | pass   |
| 2026-11-12 | REQ-005 | No network configuration is changed.                                         | The gateway comes from the read-only route adapter and is probed with ICMP echo; no write or configuration call exists.                                                                                                         | pass   |
| 2026-11-12 | REQ-010 | Probes go only to configured anchors and documented destinations.            | `NewTCPChecks` iterates `domain.BaselineAnchors()` and the documented ports 443 and 53; no call site accepts an arbitrary address.                                                                                              | pass   |
| 2026-11-12 | REQ-010 | Per-probe timeouts are bounded.                                              | Every check uses `net.Dialer` with a 2 s default timeout and the caller context, so no connect can hang the run.                                                                                                                | pass   |
| 2026-11-12 | REQ-011 | No authentication or configuration change is performed.                      | The check issues one unauthenticated GET and never submits a form or changes settings; tests assert the method and an empty Authorization header.                                                                               | pass   |
| 2026-11-12 | REQ-011 | The canary request is the only HTTP request made by this check.              | One `client.Do` call per run with `http.ErrUseLastResponse`; the simulated redirect test asserts the portal handler is never reached.                                                                                           | pass   |
| 2026-11-12 | REQ-012 | The probe is read-only.                                                      | It opens a raw ICMP socket for the probe, sets DF on that socket only, and never writes interface MTU, route or host settings.                                                                                                  | pass   |
| 2026-11-12 | REQ-012 | Packet sizes stay within the documented test range.                          | `MTUOptions.normalized` clamps sizes to 576–1500 and falls back to the documented defaults; a test rejects out-of-range input.                                                                                                  | pass   |
| 2026-11-12 | REQ-013 | The sample is bounded and cannot extend the baseline beyond its budget.      | `Count` is capped at `MaxICMPSamples` (10), each exchange uses the 1 s timeout, and the statistics reuse the REQ-009 sample without adding probes.                                                                              | pass   |
| 2026-11-12 | REQ-014 | The user can quit immediately at any point.                                  | `q` quits from dashboard, detail and help; Ctrl-C asks once and then quits; no state blocks the quit path.                                                                                                                      | pass   |
| 2026-11-12 | REQ-014 | No probe can hang the session beyond its timeout.                            | Every check runs under an 8 s per-check context and the run under a 30 s budget; a probe that ignores its context yields a timeout result and the runner moves on.                                                              | pass   |
| 2026-11-12 | REQ-014 | Completed checks remain visible and exportable after cancellation.           | `CancelAll` stops the current check, fills the remaining entries with skipped results and keeps the run as the session current run, which the dashboard renders.                                                                | pass   |
| 2026-11-12 | REQ-015 | macOS and Linux are the only v1 platforms.                                   | `cmd/egress` is build-tagged `darwin                                                                                                                                                                                            |        | linux`, so Windows cannot produce the binary; outside those systems `platform.System()`reports no ICMP capability and every route or resolver call returns`ErrUnsupported`. | pass |
| 2026-11-12 | REQ-015 | Platform-specific behavior never changes a check meaning silently.           | Probes obtain interfaces, routes, resolvers and ICMP through `platform.Capabilities`; unsupported mechanisms become skipped results with a reason, and `local.interfaces` records the platform name in its evidence.            | pass   |
| 2026-11-12 | REQ-016 | Unprivileged execution is the default.                                       | `DetectPrivilege` reports elevated only when `os.Geteuid() == 0`; `BaselineChecks` defaults to `Elevated: false`, and `cmd/egress` passes the detection result without prompting.                                               | pass   |
| 2026-11-12 | REQ-016 | Elevated access is explicit and user-initiated.                              | Raw sockets are enabled only by the detected euid; there is no sudo, setuid or re-exec path, and raw-only checks skip with an elevation hint.                                                                                   | pass   |
| 2026-11-12 | REQ-017 | Elevation is explicit.                                                       | The session mode comes only from `DetectPrivilege`; no probe or UI action can raise privileges.                                                                                                                                 | pass   |
| 2026-11-12 | REQ-017 | No check silently changes meaning between modes.                             | `BaselineChecks` returns identical ids and order in both modes, covered by a stability test; raw-only checks switch between a run and a skipped result with a stated reason.                                                    | pass   |
| 2026-11-12 | REQ-019 | Missing connectivity is never presented as censorship.                       | `ClassifyRun` uses only local-link, gateway, dns and upstream scopes with evidence-based summaries; a test rejects censorship and blockage claims.                                                                              | pass   |
| 2026-11-12 | REQ-020 | Diagnostic probes are the only outbound traffic.                             | One catalog in `internal/probe/destinations.go` is built from the probe constants; guard tests assert it covers every probe, stays closed and validates; `docs/outbound-destinations.md` and the `d` page render the same list. | pass   |
| 2026-11-12 | REQ-020 | No telemetry or enrichment API is added.                                     | The module has no analytics, crash-report, update or enrichment dependency; HTTP exists only in the DoH comparison and the captive canary.                                                                                      | pass   |
| 2026-11-12 | REQ-022 | No secrets or unrelated host data are exported.                              | The envelope carries results, verdict, classification and the documented destination list only; it has no environment, credential or file-system fields.                                                                        | pass   |
| 2026-11-12 | REQ-022 | Non-interactive output contains no terminal control sequences.               | JSON mode never builds the dashboard; the CLI test asserts stdout has no ESC byte and the output is valid JSON.                                                                                                                 | pass   |
| 2026-11-12 | REQ-023 | Context is informational only.                                               | `platform.DetectNetworkContext` reads interfaces, default routes and proxy variables; no probe, verdict or destination consumes it, and no bypass or A/B path exists.                                                           | pass   |

# Evidence Matrix

| REQ     | verification.method | evidence artifact                                                           | status (open/done/waived) |
| :------ | :------------------ | :-------------------------------------------------------------------------- | :------------------------ |
| REQ-001 | test                | `evidence/REQ-001-unit-tests.txt`; `evidence/REQ-001-schema-validation.txt` | open                      |
| REQ-003 | inspection          | `evidence/REQ-003-anchor-review.md`; `evidence/REQ-003-traceability.md`     | open                      |
| REQ-008 | test                | `evidence/REQ-008-unit-tests.txt`; `evidence/REQ-008-traceability.md`       | open                      |

REQ-001 stays open: the result model, validation and JSON envelope are proven,
but the acceptance criterion "The TUI and JSON output expose the same evidence
fields" needs the TUI adapter from REQ-021 and REQ-025.

REQ-003 stays open: the catalog, its documentation and the traceability matrix
exist, and `gothchibjo` signed off the destination list on 2026-11-12, but
anchor visibility needs the TUI and JSON adapters (REQ-025, REQ-022) and the
single-failing-anchor warning needs the REQ-018 integration.

REQ-008 stays open: detection, source reporting, explicit absence and credential
redaction are covered by unit tests, but acceptance criterion "Detection is
present in the TUI and JSON output" needs REQ-023 and REQ-022.

# Plan Revision 2 — TUI-First Execution Order

Supersedes the order in "Execution Order" for the remainder of the work. The
frozen REQ texts, acceptance criteria and guard conditions are unchanged.

## Rationale

REQ-001 and REQ-002 are open only on their TUI-facing criteria. Building the
dashboard shell and keyboard layer before the probe set closes both REQs, gives
the probe adapters a real integration target and makes progress observable
instead of landing invisible modules.

## Revised order

1. **REQ-003** — anchor set (must, xs). Independent domain data.
2. **REQ-008** — proxy environment detection (must, xs). Independent.
3. **REQ-018** — aggregate verdict (must, m), built against frozen result
   fixtures. The live-check integration of REQ-004…REQ-013 follows later; the
   REQ stays open until that integration evidence exists.
4. **REQ-025** — dashboard rendering and adaptive layout (must, l), driven by a
   run provider that accepts fixture and live results.
5. **REQ-021** — keyboard navigation and help overlay (must, m).
6. **REQ-002 wiring** — `cmd/egress/main.go`, TTY detection, session start,
   dashboard hand-off and clean exit. Then re-verify REQ-002 and REQ-001.
7. **REQ-004…REQ-013** — probe set in frozen REQ order.
8. **REQ-014** — time budget, timeouts and cancellation.
9. **REQ-015…REQ-017** — platform abstraction, unprivileged execution, elevated
   mode.
10. **REQ-019** — offline and degraded classification.
11. **REQ-020** — local-only egress policy.
12. **REQ-022** — JSON and non-interactive output.
13. **REQ-023** — proxy and VPN context display.
14. **REQ-024** — README and operational documentation.
15. **Complete** — re-run verification for every REQ, including the TUI parity
    of REQ-001 and the dashboard hand-off of REQ-002.

## TUI dependency decision

REQ-025 and REQ-021 may add `github.com/charmbracelet/bubbletea`,
`github.com/charmbracelet/lipgloss` and `github.com/charmbracelet/bubbles` for
terminal rendering, keyboard input and resize handling. No domain package
imports them.

## Entry-point task (REQ-002)

`cmd/egress/main.go` is created after REQ-025 and REQ-021: detect a TTY, build
the check list, start one session, render the dashboard and exit cleanly. It is
recorded as part of REQ-002, not as a new requirement.

# Revisions

- 2026-11-11 — draft v1 authored from frozen `03-technical.yaml` (analyst).
- 2026-11-11 — plan approved by gothchibjo; `plan_approval` gate passed.
- 2026-11-11 — REQ-001 evidence uses the test-only `jsonschema` validator to
  produce the schema report; it is not a runtime dependency.
- 2026-11-11 — REQ-002 implements the orchestration runner; the `cmd/egress`
  entry point stays pending until a dashboard adapter exists.
- 2026-11-11 — execution order revised to bring REQ-018, REQ-025, REQ-021 and
  the entry point before the probe set; approved by gothchibjo in chat. Revision
  1 order remains above.
- 2026-11-11 — Plan Revision 2 approved explicitly by gothchibjo;
  `plan_approval` remains passed.
- 2026-11-12 — REQ-003 and REQ-008 evidence recorded; both stay open on their
  TUI and JSON acceptance criteria.
- 2026-11-12 — REQ-018 implements the aggregate verdict and documents the rule
  in `docs/verdict-rule.md`; it stays open until the rule review sign-off is
  recorded.
- 2026-11-12 — REQ-003 anchor list signed off by `gothchibjo`; the requirement
  stays open on TUI and JSON visibility only.
- 2026-11-12 — REQ-025 implements the master-detail dashboard in `internal/tui`
  with path-layer grouping, adaptive layouts, resize handling and color,
  monochrome and ASCII rendering; it stays open until the TUI walkthrough and
  terminal matrix are recorded.
- 2026-11-12 — REQ-021 adds navigation, check detail, help overlay and the
  Ctrl-C confirmation to the dashboard; `docs/key-map.md` records the key map
  for review.
- 2026-11-12 — REQ-004 adds the read-only link and default-route adapter and the
  `local.interfaces` check; parser fixtures cover both v1 platforms and a
  missing route is reported as a distinct failure.
- 2026-11-12 — REQ-006 adds the resolver configuration adapter and the
  `dns.canary` check; NXDOMAIN, SERVFAIL and timeouts stay distinguishable and a
  missing resolver configuration fails with a reason.
- 2026-11-12 — REQ-007 compares the system answer with one public DoH probe,
  records both answer sets and flags differences with the CDN and geo-DNS
  caveat; a DoH failure stays separate from the system answer.
- 2026-11-12 — REQ-009 adds the ICMP adapter and per-destination checks for the
  gateway and each anchor with a bounded sample, loss, round-trip times and an
  elevation hint; REQ-005 reuses the same adapter as `gateway.reach`.
- 2026-11-12 — REQ-010 adds `tcp.anchor.<id>.<port>` checks for ports 443 and 53
  with connect latency and refused, timeout and reset classes; active rejection
  and passive timeout are labelled distinctly.
- 2026-11-12 — REQ-011 adds `http.captive` with one unauthenticated GET,
  captured redirects and a body fingerprint; the expected response is documented
  in `docs/captive-portal-canary.md`.
- 2026-11-12 — REQ-012 adds `mtu.pmtu` with a raw DF probe in elevated mode, the
  documented 576–1500 size range and skipped reasons for unprivileged or
  unreachable paths; simulated tests cover clean, blackhole and skip cases.
- 2026-11-12 — REQ-013 adds sample size, median and jitter to every ICMP result,
  derived from the existing bounded sample; odd, even, single-response and
  all-loss cases are covered by tests.
- 2026-11-12 — REQ-002 wiring adds `cmd/egress` with terminal detection, the
  full baseline check list, one session and the dashboard hand-off; progress
  from slow checks appears in the footer. Non-interactive output stays REQ-022.
- 2026-11-12 — REQ-014 adds the 30 s budget, the 8 s per-check timeout and
  `x`/`X` cancellation; Ctrl-C confirms, cancels and quits, cancelled runs are
  labelled and partial results stay visible.
- 2026-11-12 — REQ-015 adds the `platform.Capabilities` abstraction with darwin
  and linux implementations, skips unsupported mechanisms with a stated reason,
  records the platform in local evidence and excludes other OSes from the
  command build.
- 2026-11-12 — REQ-016 adds `platform.DetectPrivilege`, keeps unprivileged
  execution as the default and skips raw-only checks with a capability and
  elevation hint; there is no prompt or re-exec.
- 2026-11-12 — REQ-017 labels the session mode in the dashboard header and keeps
  check ids and meanings identical between unprivileged and elevated runs.
- 2026-11-12 — REQ-019 classifies each run as local-link, gateway, dns or
  upstream from the collected checks, shows the scope and a locally-testable
  summary in the dashboard, and keeps the wording free of censorship claims.
- 2026-11-12 — REQ-019 renders the classification sentence as a rule-bounded
  band with wrapping and a two-line cap, one line in the compact layout, so it
  no longer floats between the header and the result list or clips mid-word.
- 2026-11-12 — REQ-020 centralizes outbound access in one documented catalog,
  renders it on the `d` page, carries it in the JSON envelope and records the
  call-site inventory and capture procedure as review evidence.
- 2026-11-12 — REQ-022 adds `--json` and non-interactive stdout, `--schema`, the
  documented exit codes and the `e`/`E` export with overwrite confirmation; the
  envelope now carries mode, verdict, classification and destinations.
- 2026-11-12 — REQ-021 keeps long evidence reachable: the dashboard pane marks
  truncation, the detail page scrolls with arrows, `PgUp`/`PgDn` and `g`/`G`,
  and `h`/`l` switch checks instead of moving an invisible selection.
- 2026-11-12 — REQ-023 reports proxy variables and tunnel-like interfaces as
  read-only context on the `c` page and in the JSON envelope, with sources,
  explicit absence, redacted proxy credentials and no effect on probes.
- 2026-11-12 — REQ-023 prints the provenance rule once in the context preamble
  instead of repeating it per interface; the `[default route]` marker carries
  the only difference the TUI needs, and the JSON keeps the full source per
  entry.
