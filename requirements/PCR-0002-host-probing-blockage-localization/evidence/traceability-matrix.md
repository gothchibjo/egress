# PCR-0002 traceability matrix

Generated: 2026-11-19

The frozen `03-technical.yaml` is the contract. Each REQ maps to one business
requirement; BR-to-vision and clarification traces are recorded in
`02-business.md`. Evidence files live beside this matrix.

| REQ     | BR      | Evidence                                                                                        | Status            |
| :------ | :------ | :---------------------------------------------------------------------------------------------- | :---------------- |
| REQ-001 | [BR-07] | `REQ-001-catalog-review.md`, this matrix                                                        | evidence complete |
| REQ-002 | [BR-01] | `REQ-002-walkthrough.txt`, this matrix                                                          | evidence complete |
| REQ-003 | [BR-02] | `REQ-003-unit-tests.txt`, `REQ-003-walkthrough.txt`                                             | evidence complete |
| REQ-004 | [BR-03] | `REQ-004-ladder-tests.txt`, `REQ-004-ladder-integration.txt`, this matrix                       | evidence complete |
| REQ-005 | [BR-11] | `REQ-005-trace-tests.txt`, `REQ-005-udp-trace-integration.txt`, `REQ-005-elevated-mode.txt`     | evidence complete |
| REQ-006 | [BR-04] | `REQ-006-simulated-trace-tests.txt`, this matrix                                                | evidence complete |
| REQ-007 | [BR-05] | `REQ-007-classification-tests.txt`, `REQ-007-taxonomy-review.md`                                | evidence complete |
| REQ-008 | [BR-06] | `REQ-008-unit-tests.txt`, `REQ-008-interpretation-review.md`                                    | evidence complete |
| REQ-009 | [BR-08] | `REQ-009-concurrency-tests.txt`, `REQ-009-cancellation-tests.txt`                               | evidence complete |
| REQ-010 | [BR-09] | `REQ-010-run-view-tests.txt`, `REQ-010-walkthrough.txt`, `REQ-010-terminal-matrix.md`           | evidence complete |
| REQ-011 | [BR-10] | `REQ-011-schema-validation.txt`, `REQ-011-run-view-export-tests.txt`, `REQ-011-walkthrough.txt` | evidence complete |
| REQ-012 | [BR-12] | `REQ-012-network-review.md`, `REQ-012-code-review.md`, `REQ-012-destination-tests.txt`          | evidence complete |
| REQ-013 | [BR-13] | `REQ-013-unit-tests.txt`, `REQ-013-walkthrough.txt`                                             | evidence complete |
| REQ-014 | [BR-14] | `REQ-014-unit-tests.txt`, `REQ-014-walkthrough.txt`, `REQ-014-review.md`                        | evidence complete |
| REQ-015 | [BR-15] | `REQ-015-review.md`                                                                             | evidence complete |
| REQ-016 | [BR-16] | `REQ-016-unit-tests.txt`, `REQ-016-walkthrough.txt`, `REQ-016-keymap-review.md`                 | evidence complete |

## Notes

- REQ-005: the unprivileged path is exercised live; the elevated branch selects
  the raw ICMP tracer only when the socket opens. The review host has no
  passwordless sudo, so a live elevated trace is not in the evidence; the
  selection/fallback test and code inspection cover the branch.
- REQ-012: a root packet capture was unavailable for the same reason. The
  network review uses a live socket inventory of the probe process plus the
  call-site scan.
- Review sign-offs in the evidence files are pending the human reviewer. The
  matrix records that the evidence exists and passes; it does not sign on the
  reviewer's behalf.

<!-- refs -->

[BR-01]: ../02-business.md
[BR-02]: ../02-business.md
[BR-03]: ../02-business.md
[BR-04]: ../02-business.md
[BR-05]: ../02-business.md
[BR-06]: ../02-business.md
[BR-07]: ../02-business.md
[BR-08]: ../02-business.md
[BR-09]: ../02-business.md
[BR-10]: ../02-business.md
[BR-11]: ../02-business.md
[BR-12]: ../02-business.md
[BR-13]: ../02-business.md
[BR-14]: ../02-business.md
[BR-15]: ../02-business.md
[BR-16]: ../02-business.md
