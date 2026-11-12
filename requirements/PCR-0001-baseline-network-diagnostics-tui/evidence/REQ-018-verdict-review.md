# REQ-018 — Verdict rule review

Frozen requirement: `03-technical.yaml`, REQ-018 "Aggregate verdict logic". Test
evidence: unit test report and verdict rule review sign-off.

## Reviewed artifacts

- `docs/verdict-rule.md` — the documented deterministic rule.
- `internal/domain/verdict.go` — `AggregateVerdict` and the finding selection.
- `evidence/REQ-018-verdict-tests.txt` — 15 passing tests.

## Checklist

| Criterion (frozen)                                              | State | Where                                                                 |
| :-------------------------------------------------------------- | :---- | :-------------------------------------------------------------------- |
| The aggregation rule is documented.                             | pass  | `docs/verdict-rule.md`                                                |
| Status counts and severity are derived from the same rule.      | pass  | `Counts` is filled in the same loop that computes the level           |
| The most severe finding is identifiable without opening checks. | pass  | `Verdict.Finding`, rendered in JSON and used by the detail            |
| No verdict is produced when no check has run.                   | pass  | `ErrNoResults`, `TestAggregateVerdictRequiresResults`                 |
| One failing anchor does not produce a global blocked verdict.   | pass  | anchor downgrade rule, isolated-anchor case in `TestAggregateVerdict` |

## Review notes

- The rule is deterministic for identical input: the finding keeps the first
  result at the highest effective severity, covered by
  `TestAggregateVerdictIsDeterministic`.
- A failing anchor is downgraded to `degraded` unless at least two distinct
  anchors were checked and every one of them failed.
- An all-skipped run is `degraded`, never `healthy`.

## Sign-off

Reviewer: gothchibjo, 2026-11-12. The rule matches the frozen REQ-018 text.
