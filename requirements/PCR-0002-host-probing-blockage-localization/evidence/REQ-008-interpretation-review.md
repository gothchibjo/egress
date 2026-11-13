# REQ-008 — Interpretation review

Frozen requirement: `03-technical.yaml`, REQ-008 "Active-versus-passive evidence
interpretation". Test evidence: unit test report and interpretation review
sign-off.

## Reviewed artifacts

- `internal/domain/hypothesis.go` — observation modes, confidence floor and DNS
  caveats.
- `internal/domain/hypothesis_test.go` — mode, timeout, SNI, DNS and RST tests.
- `docs/blockage-taxonomy.md` — the documented interpretation rules.
- `evidence/REQ-008-unit-tests.txt` — 19 passing tests.

## Checklist

| Criterion (frozen)                                         | State | Where                                                                                      |
| :--------------------------------------------------------- | :---- | :----------------------------------------------------------------------------------------- |
| Every evidence line is labelled active or passive.         | pass  | `EvidenceLine.Mode`; `Validate` rejects an unlabelled line                                 |
| A timeout-only case cannot receive high confidence.        | pass  | `newHypothesis` downgrades a high confidence without an active line                        |
| DNS verdicts show both answer sets and the CDN/geo caveat. | pass  | the DNS branch appends `system answers`, `DoH answers` and the caveat                      |
| ICMP rate limiting is named when responses are absent.     | pass  | the ip-drop branch appends `no ICMP reply; ICMP may be rate-limited or blocked`            |
| RST-based and drop-based verdicts are distinguishable.     | pass  | reset is `tcp-rst-injection` with active evidence; drop is `ip-drop` with passive evidence |

## Review notes

- Active evidence comes from a peer that answered; passive evidence is silence.
  The classifier never promotes silence to high confidence.
- The DNS branch is capped at medium and names CDN/geo differences, so a
  resolver divergence is not reported as tampering on its own.
- The caveat `Single vantage point` is attached to every hypothesis.

## Sign-off

Pending review by gothchibjo.
