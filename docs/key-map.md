---
type: reference
author: gothchibjo
date: 2026-11-12
---

# Keyboard map <!-- omit in toc -->

- [Scope](#scope)
- [Keys](#keys)
- [Pages](#pages)
- [Verdict semantics](#verdict-semantics)

## Scope

The dashboard is operable without a mouse. Every action has a key, and the same
list is rendered by the `?` overlay. Mouse support remains optional and is not
implemented. The requirement is defined by [REQ-021].

## Keys

| Context           | Key        | Action                            |
| :---------------- | :--------- | :-------------------------------- |
| Dashboard, detail | `↑`, `k`   | move selection up                 |
| Dashboard, detail | `↓`, `j`   | move selection down               |
| Dashboard, detail | `g`        | select the first check            |
| Dashboard, detail | `G`        | select the last check             |
| Dashboard         | `enter`    | open the selected check detail    |
| Running baseline  | `x`        | cancel the current check          |
| Running baseline  | `X`        | cancel the baseline, keep results |
| Detail, help      | `esc`      | go back one page                  |
| Dashboard, detail | `?`        | open help                         |
| Help              | `?`, `esc` | close help and return             |
| Dashboard, detail | `q`        | quit immediately                  |
| Dashboard, detail | `ctrl+c`   | ask for confirmation, then quit   |
| Confirmation      | `y`        | quit                              |
| Confirmation      | `n`, `esc` | cancel and stay                   |

`esc` never quits, never cancels a check and never changes the selection; on the
dashboard it does nothing. `ctrl+c` asks before quitting; `q` is the immediate
exit. `x` stops only the running check and lets the baseline continue; `X` stops
the baseline but keeps completed results and labels the run `cancelled`.

## Pages

| Page      | Content                                                |
| :-------- | :----------------------------------------------------- |
| Dashboard | path list, evidence pane, contextual footer hints      |
| Detail    | one check with its full evidence at the terminal width |
| Help      | key map and verdict semantics                          |
| Quit      | confirmation after `ctrl+c`                            |

## Verdict semantics

| Verdict    | Meaning                                                  |
| :--------- | :------------------------------------------------------- |
| `healthy`  | every check passed or was skipped                        |
| `degraded` | warnings, an isolated anchor failure, or no check passed |
| `blocked`  | a core check failed, or every checked anchor failed      |

The full rule is documented in [the verdict rule] and implemented by
`domain.AggregateVerdict`.

<!-- refs -->

[REQ-021]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
[the verdict rule]: verdict-rule.md
