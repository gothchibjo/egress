---
pcr: PCR-0003
title: Non-interactive progress indication
status: approved
revised: 2026-11-12
based_on: 01-vision.md
approved_by: gothchibjo
approved_at: 2026-11-12
revisions: []
---

# Context & Goals

When `egress` runs without the interactive dashboard (`--json`, or stdout
redirected), it currently prints nothing until the whole baseline finishes. On a
slow network that silence lasts several seconds and looks like a hang. The goal
is to show the same running-check feedback the dashboard already has, without
contaminating the JSON on stdout.

# Stakeholders

| Actor          | Interest                                                         |
| :------------- | :--------------------------------------------------------------- |
| Operator       | sees that the run is alive and which check is executing          |
| Scripts and CI | receive unchanged, parseable JSON and documented exit codes      |
| Terminal user  | gets the same wording as the interactive footer, not a new style |

# Glossary

- **Non-interactive mode** — the run that writes JSON: `--json`, or stdout is
  not a terminal.
- **Progress line** — one rewritten line on stderr showing the running check.
- **Check** — one baseline probe with a stable id, `tcp.anchor.example-com.443`
  for example.

# Assumptions Ledger

| Assumption or decision                                             | Source                   | Resolved |
| :----------------------------------------------------------------- | :----------------------- | :------- |
| The progress line goes to stderr.                                  | [CL-1 Q1]                | yes      |
| It appears only when stderr is a terminal.                         | [CL-1 Q2]                | yes      |
| It reuses the dashboard text and about a one-second cadence.       | [CL-1 Q3]                | yes      |
| It appears with the first check, not after a slow-check threshold. | [CL-1 Q4]                | yes      |
| It is erased before the JSON document is written.                  | [CL-1 Q5]                | yes      |
| Ctrl-C still emits the partial JSON and the documented exit code.  | [CL-1 Q6]                | yes      |
| The JSON envelope, its schema and the exit codes do not change.    | [V §1], existing REQ-022 | yes      |

# Non-Goals

- No change to the dashboard, its footer or its key handling.
- No progress for `--schema` or `-h`.
- No progress when stderr is redirected.
- No percentage, ETA or per-check log history beyond the running check.
- No new dependency and no change to the JSON schema or exit codes.

# Atomic Business Requirements

### BR-01 — Progress is written to stderr

- description: while a non-interactive run executes, the utility writes the
  running check to stderr and leaves stdout for the JSON document alone.
- priority: must
- rationale: the operator needs feedback; scripts need an untouched stdout ([V
  §1], [CL-1 Q1]).
- acceptance criteria (business): stdout contains only the JSON document, and
  stderr shows the current check while the run is active.
- traces to: [V §1], [CL-1 Q1]
- depends on: none

### BR-02 — The progress line is terminal-only

- description: the utility emits the progress line only when stderr is a
  terminal; redirected stderr receives nothing.
- priority: must
- rationale: CI logs and captured stderr must stay clean ([CL-1 Q2]).
- acceptance criteria (business): with stderr redirected to a file, the file is
  empty; on a terminal, the line is visible.
- traces to: [V §1], [CL-1 Q2]
- depends on: BR-01

### BR-03 — Progress mirrors the dashboard wording

- description: the progress line uses the dashboard format
  `running <check-id> (<index>/<total>) <elapsed>`, is rewritten in place about
  once per second, and starts with the first check.
- priority: must
- rationale: one established style, no second indicator design ([V §1], [CL-1
  Q3], [CL-1 Q4]).
- acceptance criteria (business): the line carries the check id, its position
  and the elapsed time; it occupies one line and starts immediately.
- traces to: [V §1], [CL-1 Q3], [CL-1 Q4]
- depends on: BR-01

### BR-04 — Completion leaves a clean terminal

- description: when the run finishes, the utility erases the progress line
  before writing the JSON document.
- priority: should
- rationale: the reader should see the result, not stale progress text ([CL-1
  Q5]).
- acceptance criteria (business): after a completed run the terminal shows the
  JSON without a residual progress line.
- traces to: [CL-1 Q5]
- depends on: BR-03

### BR-05 — Cancellation is visible and still yields JSON

- description: on Ctrl-C the run stops, the progress line reports the cancelled
  state, and the partial JSON is written with the documented exit code.
- priority: should
- rationale: the operator must see the interruption, and scripts must receive
  the partial evidence ([CL-1 Q6]).
- acceptance criteria (business): an interrupted run writes a partial document,
  returns the exit code derived from its results, and leaves no half-written
  progress line.
- traces to: [CL-1 Q6]
- depends on: BR-01
