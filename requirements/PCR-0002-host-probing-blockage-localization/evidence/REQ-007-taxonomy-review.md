# REQ-007 — Taxonomy review

Frozen requirement: `03-technical.yaml`, REQ-007 "Blockage taxonomy and ranked
hypothesis". Test evidence: classification test report and verdict rule review
sign-off.

## Reviewed artifacts

- `internal/domain/hypothesis.go` — taxonomy, ranking and validation.
- `internal/domain/hypothesis_test.go` — ten scenarios, validation and taxonomy
  closure.
- `docs/blockage-taxonomy.md` — the documented rule.
- `evidence/REQ-007-classification-tests.txt` — passing tests.

## Checklist

| Criterion (frozen)                                                                           | State | Where                                                                                      |
| :------------------------------------------------------------------------------------------- | :---- | :----------------------------------------------------------------------------------------- |
| Every target ends with a type or unknown, a confidence level and at least one evidence line. | pass  | `ClassifyTarget` always sets `Type`, `Confidence`, `Evidence`; `Validate` rejects the rest |
| A type is never shown without evidence.                                                      | pass  | `Hypothesis.Validate` requires at least one evidence line                                  |
| Rejected alternatives are available on demand.                                               | pass  | `Rejected` carries seven entries with reasons                                              |
| The single-vantage-point caveat accompanies every verdict.                                   | pass  | `SingleVantageCaveat` is set for every hypothesis                                          |
| Unknown is a valid terminal outcome.                                                         | pass  | `BlockUnknown` for no evidence and for a passing target                                    |

## Review notes

- The ranking is deterministic and documented; the same steps always produce the
  same hypothesis.
- A timeout-only case is `low` confidence; `medium` requires path corroboration
  or a second signal.
- The taxonomy is closed by `BlockageType.valid()`, and a test rejects
  `censorship` as a type.

## Sign-off

Pending review by gothchibjo.
