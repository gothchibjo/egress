---
pcr: PCR-0002
title: Interactive host probing and blockage localization
status: implementing
basis: 03-technical.yaml (frozen)
revised: 2026-11-13
approved_by: gothchibjo
approved_at: 2026-11-13
---

# Plan Invariants

- The contract is sealed: `03-technical.yaml` is `frozen` and its REQ titles,
  descriptions, acceptance criteria, dependencies, verification and guard
  conditions are not editable here. A scope deviation routes to change control
  as a new PCR via `req-workflow`, not to a plan edit.
- Every task implements an existing REQ. A missing or contradictory requirement
  is reported, never patched into the plan.
- Each REQ is worked only after `req-guards` passes its guard conditions, and is
  closed only with the evidence its section names.
- Development runs in the existing `github.com/gothchibjo/egress` module and
  reuses the PCR-0001 building blocks: domain result model, runner, platform
  capabilities, report envelope, dashboard shell.
- PCR-0001 behaviour is sealed by its own tests: the baseline dashboard, its
  keys, its JSON envelope (`result-v1`) and exit codes must stay unchanged. A
  regression there blocks the REQ being worked on.
- No new dependency is added without a stated reason in the REQ section.
- Domain types stay independent of the TUI, OS probing and I/O.

# Structural Decisions

| Decision        | Choice                                                                                                                                                      | Reason                                                                        |
| :-------------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------------------------------------------------------- |
| Entry point     | the picker opens from the PCR-0001 dashboard with `p`; `Esc` returns without running                                                                        | REQ-002 says "from the dashboard"; the baseline stays one key away            |
| Session model   | new `internal/domain/session.go`: `Session`, `Target`, `LadderStep`, `Hop`, `Hypothesis`                                                                    | REQ-004…REQ-008 need richer evidence than the baseline `Result`               |
| Probe packages  | extend `internal/probe` with `ladder.go`, `tls.go`, `http_probe.go`, `trace_udp.go`, `trace_icmp.go`, `scheduler.go` (scheduler may live in `internal/app`) | one probe package keeps the adapter boundary                                  |
| Concurrency     | `internal/app` scheduler: worker pool capped at four targets, ladder steps sequential per target                                                            | REQ-009                                                                       |
| JSON            | new `session-v1` envelope and schema in `internal/report`; `result-v1` stays untouched for the baseline CLI                                                 | REQ-011 needs targets, steps, hops; changing `result-v1` would break PCR-0001 |
| Tracing         | UDP tracepath-style by default, ICMP traceroute only when already elevated                                                                                  | REQ-005                                                                       |
| TLS probe       | ClientHello with real, decoy and absent SNI over TCP, classified by the first server record, alert or reset                                                 | REQ-004                                                                       |
| Limits          | 2 s per probe, 3 probes per hop, 30 hops, 4 targets                                                                                                         | REQ-009 bounded timeouts                                                      |
| Catalog version | constant in code, reported in every session and the export                                                                                                  | REQ-001                                                                       |

# Execution Order

Topological by frozen `dependencies`, ties by REQ id.

## REQ-001 — Static preset target catalog (must, s)

- Define the catalog type: display name, host, category, default ports and
  protocols, stable id.
- Ship the eleven entries: YouTube, Telegram, Discord, GitHub, Google,
  Cloudflare, Wikipedia, X, Tor, 1.1.1.1, 8.8.8.8.
- Add the catalog version constant and expose it on the session.
- Tests: all eleven present, every entry exposes its probed address, unique ids,
  load without file or network access.
- Guards: the catalog is static and built into the binary; no update mechanism.
- Evidence: catalog review sign-off, traceability matrix.

## REQ-002 — Target picker with multi-select and filter (must, m)

- Add a `picker` view reachable from the dashboard with `p`; `Esc` returns.
- Render categories with a visible checkbox per row, the display label and the
  probed address.
- Implement `/` filtering that preserves the current selection, `Space` toggle,
  a persistent selected count, and inline custom-target entry.
- `Enter` starts the run only with at least one target; an empty selection shows
  a concrete explanation.
- Tests at 80x24 and 60x20: checkbox state, filter keeps selection, count,
  empty-selection guard, `Esc` runs nothing.
- Guards: an empty selection cannot start a run; filtering never changes the
  selection silently.
- Evidence: TUI walkthrough notes, traceability matrix.

## REQ-003 — Custom target validation and resolution (must, s)

- Parse `host`, IPv4/IPv6 literal, optional `:port` and an optional protocol
  override; reject full URLs with scheme or path.
- Validate inline: empty host, bad literal, port outside 1-65535.
- Resolve hostnames with the system resolver before probing and show the
  resolved addresses; de-duplicate targets by resolved address and port.
- An unresolvable hostname becomes a diagnostic result, not a crash.
- Tests: valid host, IPv4, IPv6, ports 0/65536, duplicates, unresolvable host,
  URL rejection.
- Guards: URLs with scheme and path are out of v1; resolution failure is
  diagnostic.
- Evidence: unit test report, TUI walkthrough notes.

## REQ-004 — Layered probe ladder per target (must, l)

- Add the ladder orchestrator per target: DNS (system + DoH comparison), TCP
  connect on the target ports, TLS ClientHello with real/decoy/absent SNI, HTTP
  GET, ICMP echo; traceroute runs alongside.
- Model every step as an independent result with order, timestamp, duration,
  target and raw evidence; a failed step never cancels later steps.
- A DNS failure still allows TCP probes when an IP is known.
- Keep real, decoy and absent SNI outcomes distinguishable.
- Tests: unit with fakes per step; opt-in integration against the curated
  anchors.
- Guards: probes only to selected targets and declared resolvers; every step
  bounded.
- Evidence: integration test report, traceability matrix.

## REQ-005 — Privilege mode and tracing method (must, m)

- Implement UDP tracepath-style tracing for unprivileged sessions.
- Add ICMP traceroute behind the existing elevated capability check.
- Label the session `limited` or `elevated` in the run view and the export.
- Explain a missing raw-socket capability with an elevation hint; never prompt
  or escalate.
- Tests: method selection per mode, label, hint, identical result meanings.
- Guards: unprivileged default; elevation explicit.
- Evidence: unprivileged test report, elevated mode test report.

## REQ-006 — Path discovery and drop-hop localization (must, l)

- Build the hop table from trace data: per-hop sent/received, loss, min/median
  latency.
- Localize the path break as "drop after hop N" or "inconclusive between hops N
  and M"; never present a silent hop as proof of blocking.
- Cross-check the localization against TCP and DNS outcomes; ambiguous stars
  produce an inconclusive segment with alternatives.
- Simulation tests with crafted hop data: clean path, single drop, star range,
  contradictory ladder evidence.
- Guards: single vantage point; no packet capture.
- Evidence: simulated trace test report, traceability matrix.

## REQ-007 — Blockage taxonomy and ranked hypothesis (must, l)

- Define the taxonomy: DNS tampering, TCP RST injection, IP drop/blackhole, port
  block, SNI/TLS filtering, HTTP block/captive portal, throttling, unknown.
- Produce per target one ranked hypothesis with confidence, supporting evidence
  and rejected alternatives; `unknown` is a valid terminal outcome.
- Document the ranking rule in `docs/blockage-taxonomy.md`.
- Tests: one case per taxonomy member, rejected alternatives, no verdict without
  evidence, `unknown` for thin evidence.
- Guards: a verdict always carries evidence; the taxonomy is not extended.
- Evidence: classification test report, verdict rule review sign-off.

## REQ-008 — Active-versus-passive evidence interpretation (must, m)

- Label every evidence line `active` or `passive`.
- Forbid high confidence for a timeout-only case; name ICMP rate limiting when
  responses are absent.
- Show both DNS answer sets with the CDN/geo caveat; keep RST and drop verdicts
  distinguishable.
- Tests: RST vs drop, timeout-only, rate-limited ICMP, CDN answer variance,
  captive portal.
- Guards: silence is never proof of censorship; CDN differences are not
  tampering.
- Evidence: unit test report, interpretation review sign-off.

## REQ-009 — Scheduling, concurrency and cancellation (must, m)

- Add a scheduler in `internal/app`: at most four targets concurrently, ladder
  steps sequential per target, bounded probe timeout and retry count.
- Key semantics: `x` stops the current target, `X` stops all targets, `Esc`
  never cancels, `Ctrl-C` asks before quitting.
- Keep partial results exportable and the UI responsive; a hung probe cannot
  stall the session beyond its timeout.
- Tests with fake probes and a fake clock: concurrency cap, order, cancellation,
  timeout, partial results.
- Guards: concurrency capped at four; every timeout bounded.
- Evidence: concurrency test report, cancellation test report.

## REQ-010 — Adaptive run view with target tabs (must, l)

- Render target tabs with running/done/cancelled states and the views Ladder,
  Hops and Verdict.
- Wide terminals show ladder and hops side by side; narrow terminals switch
  between them. Keep the 80x24 / 60x20 / 40x10 thresholds of the existing shell.
- Reuse the status marker and colour rules: monochrome and ASCII stay readable,
  `NO_COLOR` honoured, 120/256/16-colour profiles.
- Resizing reflows without losing the selected target, tab or scroll offset.
- Help is reachable from the run view.
- Tests: layout thresholds, tab states, monochrome, ASCII, resize, transcript.
- Guards: status never depends on colour; minimum sizes enforced.
- Evidence: TUI walkthrough notes, terminal matrix.

## REQ-011 — Session JSON export (must, m)

- Add `session-v1`: targets, ladder steps, raw evidence, hops, hypothesis,
  confidence, timestamps, privilege mode, catalog version, context and
  destination list.
- Add `session-v1.schema.json` and validate every export in tests.
- `e` writes `egress-<timestamp>.json`, shows the path; `E` opens a path prompt;
  an existing file needs confirmation. Partial results after cancellation export
  the same way.
- Tests: schema validation, default name, overwrite confirmation, cancelled
  session, no secrets, no unrelated host data.
- Guards: no history or database; export contains only current-session data.
- Evidence: JSON schema validation report, TUI walkthrough notes.

## REQ-012 — Local-only operation and destination transparency (must, s)

- Build the per-run destination list: selected target addresses and declared
  resolvers, shown in the run view and included in the export.
- Confirm no telemetry, upload, enrichment or background call is added.
- Extend `docs/outbound-destinations.md` with the session destinations.
- Inspection: call-site inventory, attributed network capture, code review.
- Guards: only declared destinations receive traffic; no telemetry.
- Evidence: network capture review, code review sign-off.

## REQ-013 — Proxy and VPN context display (should, s)

- Reuse `platform.DetectNetworkContext` for proxy variables and tunnel
  interfaces.
- Show the context in the run view and the session export, with sources and an
  explicit absence state.
- Tests: detected proxy, detected tunnel, both absent, export parity.
- Guards: context is informational only; no setting is modified.
- Evidence: unit test report, TUI walkthrough notes.

## REQ-014 — Limitations and confidence disclosure (should, s)

- Add the caveat block to every verdict, results view and export: single vantage
  point, the false-positive risks of each probe, absence of evidence is not
  proof, and a suggestion to re-run from another network.
- Make the caveat impossible to hide or collapse.
- Inspection against the verdict and export output plus walkthrough.
- Guards: the caveat cannot be hidden; no claim of proof.
- Evidence: verdict review sign-off, TUI walkthrough notes.

## REQ-015 — Tool survey and feature rationale document (should, s)

- Write `docs/tool-survey.md` comparing ping, traceroute, mtr, tcptraceroute,
  nexttrace, trippy, dig/dog, curl, OONI Probe and RIPE Atlas: capability and
  privilege model.
- List adopted, deferred and rejected features with reasons, each adopted item
  linked to its BR.
- Keep the document updated when scope changes.
- Guards: English; only the surveyed tools are named.
- Evidence: documentation review sign-off.

## REQ-016 — In-app help for picker and results (could, s)

- Extend the help overlay for the picker and the run view: selection, ladder
  steps, hop-table reading, taxonomy, confidence levels and keys.
- Dismissal returns to the same state without losing the run.
- Tests: help reachable from both views, content matches the key map, run state
  preserved.
- Guards: help matches the implemented key map; dismissal never resets the run.
- Evidence: TUI walkthrough notes, key map review sign-off.

# Plan Lifecycle

## Revision 1 — 2026-11-13

Initial plan for the frozen PCR-0002 contract: 16 REQs, topological order,
`session-v1` export alongside the sealed `result-v1`, and reuse of the PCR-0001
shell, runner and platform adapters.

The plan is not approved: `plan_approval` stays `false` until a human approves
it.

## Revision 2 — 2026-11-13

The picker key is `t`, not `p`: `p` has been bound to "previous problem" on the
dashboard since PCR-0001. No requirement changes.

## Approval — 2026-11-13

Approved by gothchibjo. Revision 1 is the working plan; the 16 REQs are open for
implementation in the order above.

# Guard Log

| Date       | REQ     | Guard                                                               | Check                                                                                                                                                            | Result |
| :--------- | :------ | :------------------------------------------------------------------ | :--------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----- |
| 2026-11-13 | REQ-001 | The catalog stays static and built into the binary in v1.           | No catalog file, environment override or remote fetch exists or is planned; the catalog will be a constant slice in `internal/domain/target.go`.                 | pass   |
| 2026-11-13 | REQ-001 | Catalog entries require no remote update mechanism.                 | No update client, URL or dependency is introduced; the catalog accessor takes no parameters and performs no I/O.                                                 | pass   |
| 2026-11-13 | REQ-002 | An empty selection cannot start a run.                              | The picker is new and has no run path yet; Enter will be gated on a non-empty selection and a test asserts the start callback is not invoked.                    | pass   |
| 2026-11-13 | REQ-002 | Filtering never changes the selection silently.                     | Selection is a set keyed by preset id, independent of the filtered view; a test toggles rows, filters and re-reads the set.                                      | pass   |
| 2026-11-13 | REQ-003 | Full URLs with scheme and path remain out of v1.                    | The parser rejects any input containing `://`, `/`, `?` or `#`, and a test asserts the error message names the limitation.                                       | pass   |
| 2026-11-13 | REQ-003 | An unresolvable hostname produces a diagnostic result, not a crash. | Resolution errors are stored on the custom target and rendered as `unresolved: ...`; a test injects a failing resolver.                                          | pass   |
| 2026-11-13 | REQ-004 | Probes go only to selected targets and declared resolvers.          | The ladder takes an explicit target and uses the configured resolvers plus the documented DoH endpoint; no other destination is dialed.                          | pass   |
| 2026-11-13 | REQ-004 | Every probe step has a bounded timeout.                             | Each step runs under a 2 s context deadline; a timed-out step becomes a fail or skipped result and never blocks later steps.                                     | pass   |
| 2026-11-13 | REQ-005 | Unprivileged execution is the default.                              | `NewTracer(false)` selects UDP tracing; the ICMP tracer opens a raw socket only when the session is already elevated.                                            | pass   |
| 2026-11-13 | REQ-005 | Elevation is explicit and user-initiated.                           | No prompt, re-exec or setuid path exists; the elevated branch only reacts to the already-detected euid and falls back to UDP when the raw socket is unavailable. | pass   |
| 2026-11-13 | REQ-006 | Localization relies on a single vantage point.                      | Every localization carries the single-vantage-point alternative; no finding claims a global view.                                                                | pass   |
| 2026-11-13 | REQ-006 | No packet capture is used in v1.                                    | Path discovery uses TTL-limited probes and ICMP errors only; no capture API or pcap dependency exists.                                                           | pass   |
| 2026-11-13 | REQ-007 | A verdict is never produced without evidence.                       | `Hypothesis.Validate` rejects an empty evidence list; every classifier branch builds at least one evidence line.                                                 | pass   |
| 2026-11-13 | REQ-007 | The taxonomy is not extended without a new PCR.                     | `BlockageType.valid()` enumerates exactly the eight frozen members; an unknown type fails validation and a test rejects it.                                      | pass   |
| 2026-11-13 | REQ-008 | Silence is never presented as proof of censorship.                  | Passive evidence is labelled, high confidence requires an active observation, and every hypothesis keeps the vantage caveat.                                     | pass   |
| 2026-11-13 | REQ-008 | CDN and geo-DNS differences are not reported as tampering.          | The DNS branch is capped at medium and explicitly names CDN/geo answers as a legitimate cause of divergence.                                                     | pass   |
| 2026-11-13 | REQ-009 | Concurrency is capped at four targets.                              | `MaxConcurrentTargets = 4` bounds the worker pool; a test records a peak of at most four concurrent ladders.                                                     | pass   |
| 2026-11-13 | REQ-009 | Every probe timeout is bounded.                                     | Each ladder step runs under a 2 s deadline; a session test with a hung ladder returns after its context deadline.                                                | pass   |

# Implementation Log

- 2026-11-13 — REQ-001 ships the eleven-entry static catalog and the version
  constant in `internal/domain/target.go`, with content, copy and determinism
  tests and `docs/target-catalog.md`. The requirement stays open until the
  session export (REQ-011) reports the catalog version.
- 2026-11-13 — REQ-002 adds the target picker: categories with checkboxes, `/`
  filter that preserves the selection, `c` for an inline custom target, a live
  selected count, cursor-driven scrolling, and `enter` that refuses an empty
  selection. The picker key is `t` (Revision 2).
- 2026-11-13 — REQ-002 picker layout: one column below 100 columns, two from 100
  and three from 160; category blocks are balanced across the available columns
  and paged by whole columns so the cursor stays visible.
- 2026-11-13 — REQ-002 picker polish: rows render as `Name (host)`, and `←`/`→`
  move the cursor to the nearest entry in the adjacent column using the same
  layout the renderer uses; edges and the single-column layout do not move.
- 2026-11-13 — REQ-003 parses custom targets (`host[:port][@protocol]`), rejects
  URLs, paths and invalid ports inline, resolves hostnames off the event loop,
  shows the addresses or the resolution error, and de-duplicates by normalized
  key.
- 2026-11-13 — REQ-004 adds the independent ladder per target: DNS with DoH
  comparison, TCP per port, three SNI variants, HTTP, ICMP and a trace step that
  stays skipped until REQ-005; every step has its own 2 s timeout, timing and
  measurements.
- 2026-11-13 — REQ-005 adds UDP tracepath-style tracing for limited sessions and
  an ICMP tracer for elevated sessions, falling back to UDP with a stated reason
  when a raw socket is unavailable; the ladder's trace step keeps partial paths
  and marks truncated discovery as a warning.
- 2026-11-13 — REQ-006 adds the hop table report and `LocalizePath`: complete,
  drop after hop N, or inconclusive between hops N and M, cross-checked with the
  ladder and always carrying alternative explanations.
- 2026-11-13 — REQ-007 adds the closed taxonomy, the deterministic ranking and
  `Hypothesis` with evidence, rejected alternatives, confidence and the
  single-vantage caveat; ten scenarios cover the members and the unknown
  outcome.
- 2026-11-13 — REQ-008 labels every evidence line active or passive, forbids
  high confidence without an active observation, names ICMP rate limiting, shows
  both DNS answer sets with the CDN/geo caveat and keeps reset and drop findings
  distinct.
- 2026-11-13 — REQ-009 adds the probing session: at most four targets in
  parallel, sequential steps per target, per-target and whole-session
  cancellation, partial steps kept, and a second concurrent run rejected.
