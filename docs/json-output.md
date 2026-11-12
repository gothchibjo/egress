---
type: reference
author: gothchibjo
date: 2026-11-12
---

# JSON output and exit codes <!-- omit in toc -->

- [Invocation](#invocation)
- [Envelope](#envelope)
- [Exit codes](#exit-codes)
- [TUI export](#tui-export)

## Invocation

| Command           | Behaviour                                       |
| :---------------- | :---------------------------------------------- |
| `egress`          | interactive dashboard when stdout is a terminal |
| `egress --json`   | one JSON document on stdout, no terminal output |
| `egress --schema` | print the JSON Schema and exit                  |
| `egress -h`       | print the usage line and exit                   |

When stdout is not a terminal, `egress` runs non-interactively and writes JSON
even without `--json`. Nothing is rendered with ANSI escapes in that mode, so
the output can be piped or redirected. `Ctrl-C` stops the run and still emits
the results collected so far. The behaviour is defined by [REQ-022].

## Envelope

| Field            | Presence   | Content                                                    |
| :--------------- | :--------- | :--------------------------------------------------------- |
| `schema_version` | always     | `v1`                                                       |
| `mode`           | always     | `elevated` or `unprivileged`                               |
| `results`        | always     | one object per check, the same evidence the TUI shows      |
| `verdict`        | finished   | aggregate level, status counts and the most useful finding |
| `classification` | on failure | failing path scope and the locally testable summary        |
| `destinations`   | always     | the documented outbound endpoints                          |

Optional sections are omitted instead of written empty. The schema lives in
`internal/report/schema/result-v1.schema.json` with the identifier
`https://github.com/gothchibjo/egress/schema/result-v1.json`; `--schema` prints
it, and `internal/report` validates every document against it in tests.

## Exit codes

| Code | Meaning                                                    |
| ---: | :--------------------------------------------------------- |
|    0 | healthy: every check passed or was skipped                 |
|    1 | degraded: warnings, an isolated anchor failure, or no pass |
|    2 | blocked: a core check failed or every anchor failed        |
|    3 | usage or runtime error, including an invalid command line  |

The codes follow [the verdict rule] and apply to both the dashboard and the JSON
output. Quitting the dashboard before the first check finishes is a normal user
action and exits `0`.

## TUI export

| Key             | Action                                                   |
| :-------------- | :------------------------------------------------------- |
| `e`             | write `egress-<timestamp>.json` in the working directory |
| `E`             | open a path prompt, pre-filled with the default name     |
| `enter`         | write to the typed path                                  |
| `esc`, `ctrl+c` | close the prompt without writing                         |
| `y`             | replace an existing file after the confirmation          |
| `n`, `esc`      | keep the existing file                                   |

The timestamp is UTC in `YYYYMMDD-HHMMSS` form. The footer shows the written
path or the failure reason, and a completed or cancelled run can both be
exported. Files are created with owner-only permissions.

<!-- refs -->

[the verdict rule]: verdict-rule.md
[REQ-022]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
