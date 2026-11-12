# REQ-024 — README review

Frozen requirement: `03-technical.yaml`, REQ-024 "README and operational
documentation". Inspection evidence: documentation review sign-off.

## Reviewed artifact

`README.md` at the repository root, with the documents it links:

- `docs/outbound-destinations.md` — the local-only egress policy and the probe
  destinations.
- `docs/json-output.md` — the envelope, the exit codes and the TUI export.
- `docs/key-map.md` — every key and page.
- `docs/verdict-rule.md` — the aggregate verdict.
- `internal/report/schema/result-v1.schema.json` — the published schema.
- `requirements/PCR-0001-baseline-network-diagnostics-tui/` — the frozen
  specification and its evidence.

## Checklist

| Criterion (frozen)                                   | State | Where in the README                       |
| :--------------------------------------------------- | :---- | :---------------------------------------- |
| The README covers installation and both modes.       | pass  | Install, Privilege modes                  |
| The README documents the local-only privacy rule.    | pass  | Privacy                                   |
| The README documents JSON output and exit codes.     | pass  | JSON output, Exit codes                   |
| Keys are discoverable without reading the README.    | pass  | `?` overlay in the dashboard, `Keys` link |
| Documentation is written in English.                 | pass  | whole document                            |
| The anchor list, verdict rule and schema are linked. | pass  | Documentation table                       |

## Review notes

- Installation uses only the module path from `go.mod`; no version or release
  number is stated because none exists.
- The privilege table names `sudo egress` as an operator action, while the text
  states that the utility never escalates itself.
- The privacy section points at the destination catalog rather than repeating
  the list, so the two cannot drift.
- Exit codes match `docs/json-output.md`, the CLI implementation and the command
  tests.

## Sign-off

Reviewer: gothchibjo, 2026-11-12. The README covers installation, both privilege
modes, the local-only rule, JSON usage and exit codes, and links the anchor
list, verdict rule, schema and key map.
