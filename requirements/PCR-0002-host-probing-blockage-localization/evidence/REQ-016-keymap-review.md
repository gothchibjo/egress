# REQ-016 key map review

Date: 2026-11-19 Reviewer sign-off: pending

## Coverage

| Help section | Key map rows                                                     |
| :----------- | :--------------------------------------------------------------- |
| NAVIGATION   | arrows, j/k, g/G, h/l, [, ], n/p, PgUp/PgDn, enter, esc, c, d, t |
| ACTIONS      | r, e/E, x/X, q/ctrl+c                                            |
| PICKER       | space, a, /, c, enter, esc                                       |
| RUN          | 1/2/3, tab, enter, c/d, e/E, r                                   |
| LADDER       | every step id and the status markers                             |
| HOPS         | TTL, ADDRESS, LOSS, RTT, markers, localization outcomes          |
| TAXONOMY     | all eight taxonomy members                                       |
| CONFIDENCE   | low, medium, high, unknown                                       |
| CAVEAT       | single vantage point, absence of evidence, re-run suggestion     |
| VERDICT      | healthy, degraded, blocked                                       |

## State preservation

TestHelpKeepsThePickerState keeps the selected preset and cursor;
TestHelpKeepsTheRunState keeps the selected target and the hops pane. Both close
help with esc or ? and never restart the session.

## Key map

docs/key-map.md lists the same keys. Picker `a` and run `c` were added to the
table so the document matches the handlers.
