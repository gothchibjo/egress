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

| Context           | Key             | Action                             |
| :---------------- | :-------------- | :--------------------------------- |
| Dashboard, detail | `↑`, `k`        | move selection up                  |
| Dashboard, detail | `↓`, `j`        | move selection down                |
| Dashboard, detail | `g`             | select the first check             |
| Dashboard, detail | `G`             | select the last check              |
| Dashboard         | `enter`         | open the selected check detail     |
| Dashboard         | `e`             | write `egress-<timestamp>.json`    |
| Dashboard         | `E`             | choose the export path             |
| Export prompt     | `enter`         | write the file                     |
| Export prompt     | `esc`, `ctrl+c` | close the prompt without writing   |
| Overwrite prompt  | `y`             | replace the existing file          |
| Overwrite prompt  | `n`, `esc`      | keep the existing file             |
| Dashboard, detail | `d`             | open the outbound destination list |
| Destinations      | `d`, `esc`      | go back to the previous page       |
| Destinations      | `↑`, `k`        | scroll the destination table up    |
| Destinations      | `↓`, `j`        | scroll the destination table down  |
| Destinations      | `g`, `G`        | jump to the first or last row      |
| Running baseline  | `x`             | cancel the current check           |
| Running baseline  | `X`             | cancel the baseline, keep results  |
| Detail, help      | `esc`           | go back one page                   |
| Dashboard, detail | `?`             | open help                          |
| Help              | `?`, `esc`      | close help and return              |
| Dashboard, detail | `q`             | quit immediately                   |
| Dashboard, detail | `ctrl+c`        | ask for confirmation, then quit    |
| Confirmation      | `y`             | quit                               |
| Confirmation      | `n`, `esc`      | cancel and stay                    |

`esc` never quits, never cancels a check and never changes the selection; on the
dashboard it does nothing. `ctrl+c` asks before quitting; `q` is the immediate
exit. `x` stops only the running check and lets the baseline continue; `X` stops
the baseline but keeps completed results and labels the run `cancelled`.

The evidence pane on the dashboard truncates long evidence and ends with
`… enter for the full evidence`. The detail page keeps the whole record, shows
the visible line range in the header and scrolls instead of truncating; `h`/`l`
change the selected check and reset the scroll to the top.

The `d` page lists the documented outbound endpoints from [the outbound
destination list]. It scrolls with `j`/`k` and `g`/`G` and returns to the page
it was opened from.

## Pages

| Page          | Content                                                |
| :------------ | :----------------------------------------------------- |
| Dashboard     | path list, evidence pane, contextual footer hints      |
| Detail        | one check with its full evidence at the terminal width |
| Help          | key map and verdict semantics                          |
| Destinations  | documented outbound endpoints and their transport      |
| Export prompt | default export path with overwrite confirmation        |
| Quit          | confirmation after `ctrl+c`                            |

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
[the outbound destination list]: outbound-destinations.md
