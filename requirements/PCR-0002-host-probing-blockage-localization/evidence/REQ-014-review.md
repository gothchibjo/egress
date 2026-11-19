# REQ-014 verdict review

Date: 2026-11-19 Reviewer sign-off: pending

## What was checked

- The caveat appears in the always-visible verdict band and in the VERDICT pane;
  the pane places CAVEAT before EVIDENCE so a short terminal shows it first.
- The band wraps the caveat instead of clipping it. The review transcript covers
  120x40, 82x24 and the compact 60x20 verdict.
- The caveat names the probes that ran: DNS (CDN/geo), TCP (middleboxes), TLS
  (policy/certificates), ICMP/trace (rate limits) and HTTP (application policy).
- Every caveat states that absence of evidence is not proof of blocking and
  suggests re-running from another network.
- session-v1 exports the same caveat string for every target.

## No-proof scan

```text
$ grep -rn "proof of blocking" internal/domain internal/tui
internal/domain/hypothesis.go:73:const SingleVantageCaveat = "Single vantage point: one observation from one network; absence of evidence is not proof of blocking."
internal/domain/hypothesis.go:106:		". Absence of evidence is not proof of blocking. Re-run from another network to validate."
internal/domain/path.go:51:// never presented as proof of blocking: every finding carries the alternative
```

The only occurrence is the disclosure that absence of evidence is not proof.
Classifier summaries use observations ("no reply", "timed out", "differ"), never
a claim of proof.

## Unknown preference

TestClassifyTargetScenarios covers thin-evidence targets and expects
BlockUnknown with low confidence. Unknown remains a valid terminal outcome.
