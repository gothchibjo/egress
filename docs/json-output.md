---
type: reference
author: gothchibjo
date: 2026-11-12
---

# JSON output and exit codes <!-- omit in toc -->

- [Invocation](#invocation)
- [Envelope](#envelope)
- [Session export](#session-export)
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
| `cancelled`      | on cancel  | `true` when the run was stopped early                      |
| `results`        | always     | one object per check, the same evidence the TUI shows      |
| `verdict`        | finished   | aggregate level, status counts and the most useful finding |
| `classification` | on failure | failing path scope and the locally testable summary        |
| `context`        | always     | proxy and tunnel context with its source                   |
| `destinations`   | always     | the documented outbound endpoints                          |

Optional sections are omitted instead of written empty. The schema lives in
`internal/report/schema/result-v1.schema.json` with the identifier
`https://github.com/gothchibjo/egress/schema/result-v1.json`; `--schema` prints
it, and `internal/report` validates every document against it in tests.

## Session export

The run view writes a probing session as `session-v1`. The schema lives in
`internal/report/schema/session-v1.schema.json` with the identifier
`https://github.com/gothchibjo/egress/schema/session-v1.json`.

| Field             | Presence  | Content                                                      |
| :---------------- | :-------- | :----------------------------------------------------------- |
| `schema_version`  | always    | `session-v1`                                                 |
| `started_at`      | always    | session start in RFC 3339                                    |
| `finished_at`     | always    | session finish in RFC 3339                                   |
| `mode`            | always    | `elevated` or `unprivileged`                                 |
| `catalog_version` | always    | version of the built-in preset catalog                       |
| `cancelled`       | on cancel | `true` when the session stopped early                        |
| `targets`         | always    | id, host, state, steps, path and hypothesis for every target |
| `destinations`    | always    | documented outbound endpoints relevant to the session        |
| `context`         | always    | proxy and tunnel context with its source                     |

Each target carries the ladder steps verbatim, the hop table, the path
localization and the ranked hypothesis with its confidence, evidence and caveat.
A cancelled session keeps the steps each target had already collected.

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
| `e` (run view)  | write the `session-v1` export for the current session    |
| `E`             | open a path prompt, pre-filled with the default name     |
| `E` (run view)  | open the prompt for the `session-v1` export              |
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
