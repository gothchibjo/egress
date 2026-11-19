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

| Context               | Key              | Action                                      |
| :-------------------- | :--------------- | :------------------------------------------ |
| Dashboard             | `↑`, `k`         | move selection up                           |
| Dashboard             | `↓`, `j`         | move selection down                         |
| Dashboard             | `g`, `G`         | select the first or last check              |
| Dashboard             | `enter`          | open the selected check detail              |
| Dashboard             | `e`              | write `egress-<timestamp>.json`             |
| Dashboard             | `E`              | choose the export path                      |
| Export prompt         | `enter`          | write the file                              |
| Export prompt         | `esc`, `ctrl+c`  | close the prompt without writing            |
| Overwrite prompt      | `y`              | replace the existing file                   |
| Overwrite prompt      | `n`, `esc`       | keep the existing file                      |
| Dashboard, detail     | `d`              | open the outbound destination list          |
| Dashboard, detail     | `c`              | open the proxy and tunnel context           |
| Dashboard             | `t`              | open the target picker                      |
| Picker                | `↑`, `k`         | move the cursor up                          |
| Picker                | `↓`, `j`         | move the cursor down                        |
| Picker                | `g`, `G`         | jump to the first or last row               |
| Picker                | `←`, `→`         | move to the previous or next column         |
| Picker                | `space`          | select a preset or remove a custom          |
| Picker                | `/`              | filter presets                              |
| Picker filter         | `enter`, `esc`   | apply or clear the filter                   |
| Picker                | `c`              | add a custom target                         |
| Picker custom         | `enter`, `esc`   | add or cancel the custom target             |
| Picker                | `enter`          | start when at least one target is selected  |
| Picker                | `esc`            | return without running                      |
| Run                   | `↑`/`k`, `↓`/`j` | previous or next target                     |
| Run                   | `h`/`l`, `[`/`]` | previous or next target                     |
| Run                   | `1`, `2`, `3`    | select the ladder, hops or verdict pane     |
| Run                   | `tab`            | cycle the pane                              |
| Run                   | `enter`          | focus the selected pane for scrolling       |
| Run pane              | `↑`/`k`, `↓`/`j` | scroll one line                             |
| Run pane              | `PgUp`, `PgDn`   | scroll one page                             |
| Run pane              | `g`, `G`         | jump to the top or end                      |
| Run pane              | `h`/`l`, `[`/`]` | previous or next target                     |
| Run pane              | `esc`            | leave the pane, stay in the run             |
| Run                   | `e`, `E`         | export the session to default or typed path |
| Run                   | `r`              | run the current target selection again      |
| Run                   | `x`, `X`         | cancel the current or every target          |
| Run                   | `esc`            | back to the dashboard, never cancels        |
| Running baseline      | `x`              | cancel the current check                    |
| Running baseline      | `X`              | cancel the baseline, keep results           |
| Detail, help, context | `esc`            | go back one page                            |
| Dashboard, detail     | `?`              | open help                                   |
| Help                  | `?`, `esc`       | close help and return                       |
| Dashboard, detail     | `q`              | quit immediately                            |
| Dashboard, detail     | `ctrl+c`         | ask for confirmation, then quit             |
| Confirmation          | `y`              | quit                                        |
| Confirmation          | `n`, `esc`       | cancel and stay                             |

`esc` never quits, never cancels a check and never changes the selection; on the
dashboard it does nothing. `ctrl+c` asks before quitting; `q` is the immediate
exit. `x` stops only the running check and lets the baseline continue; `X` stops
the baseline but keeps completed results and labels the run `cancelled`.

A `warn` or `fail` result marks its `diagnostic` and `error` lines with `!` in
the left gutter, so the line explaining the status is visible without colour.

The evidence pane on the dashboard truncates long evidence and ends with
`... enter for the full evidence`. The detail page keeps the whole record, shows
the visible line range in the header and scrolls instead of truncating; `h`/`l`
change the selected check and reset the scroll to the top.

The `d` page lists the documented outbound endpoints from [the outbound
destination list]. The `c` page shows the proxy and tunnel context described in
[the network context]. Both scroll with `j`/`k` and `g`/`G` and return to the
page they were opened from.

In the run view, `e` and `E` write the session export (`session-v1`) instead of
the baseline export. `r` replaces the finished target results with a new run of
the same selection; it is refused while a probe session is still active. On a
narrow terminal the ladder and the hops share the right column, stacked
vertically, instead of being squeezed into three columns.

`n` and `p` walk only through checks with status `warn` or `fail`, in the order
they appear in the list, and wrap at both ends. `r` runs the baseline again; the
new run replaces the previous results instead of merging with them, and the
footer reports `baseline is already running` while a run is active.

The footer status slot shows progress, the selection or a transient message such
as `wrote egress-<timestamp>.json`. A message disappears after four seconds or
on the next action, and it never adds a row to the footer.

## Pages

| Page          | Content                                                   |
| :------------ | :-------------------------------------------------------- |
| Dashboard     | path list, evidence pane, contextual footer hints         |
| Detail        | one scrollable check with its full evidence               |
| Help          | scrollable key map and verdict semantics                  |
| Destinations  | documented outbound endpoints and their transport         |
| Context       | proxy and tunnel context with its source                  |
| Picker        | target selection with categories, filter and custom input |
| Run           | target tabs, ladder, hops and verdict                     |
| Export prompt | default export path with overwrite confirmation           |
| Quit          | confirmation after `ctrl+c`                               |

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
[the network context]: network-context.md
