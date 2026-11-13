---
type: reference
author: gothchibjo
date: 2026-11-13
---

# Blockage taxonomy <!-- omit in toc -->

- [Members](#members)
- [Ranking](#ranking)
- [Active and passive evidence](#active-and-passive-evidence)
- [Confidence](#confidence)
- [Clean targets](#clean-targets)
- [Caveats](#caveats)

## Members

The taxonomy is fixed by the frozen contract. Adding a member requires a new
PCR.

| Type                | Meaning                                                                       |
| :------------------ | :---------------------------------------------------------------------------- |
| `dns-tampering`     | the system resolver and DoH disagree, or the resolver fails while DoH answers |
| `tcp-rst-injection` | a TCP connect is reset by the peer                                            |
| `ip-drop`           | every TCP probe times out and the path shows no reply after the last hop      |
| `port-block`        | some ports connect while others time out on the same target                   |
| `sni-tls-filtering` | TLS fails with the real SNI but succeeds with a decoy or no SNI               |
| `http-block`        | the HTTP step fails or returns a rejection status while transport works       |
| `throttling`        | latency evidence above the threshold (not produced by the current probes)     |
| `unknown`           | no specific type matches, or no evidence exists at all                        |

## Ranking

The classifier evaluates the rules in this order and returns the first match:

1. SNI/TLS filtering
2. DNS tampering
3. TCP RST injection
4. Port block
5. HTTP block
6. IP drop
7. Throttling
8. Unknown

Specific evidence wins over generic timeouts: a real-SNI failure with a working
decoy is stronger than a timeout, and a reset is stronger than a refused port.

## Active and passive evidence

Every evidence line carries a mode:

| Mode      | Meaning                                                                                          |
| :-------- | :----------------------------------------------------------------------------------------------- |
| `active`  | the peer answered, rejected or reset: a reset, a refusal, an HTTP status, a differing DNS answer |
| `passive` | a timeout or silence: no reply, no ICMP, a silent hop                                            |

Consequences:

- high confidence requires at least one active observation; a timeout-only case
  is capped at medium and normally reported as low;
- ICMP absence is labelled passive and names rate limiting as the likely cause;
- DNS divergence lists the system and DoH answer sets and adds the CDN/geo
  caveat, and never exceeds medium;
- a reset (active) and a drop (passive) are different types and are never
  presented as the same finding.

## Confidence

| Level    | Basis                                                                             |
| :------- | :-------------------------------------------------------------------------------- |
| `high`   | two independent signals agree, for example a failing real SNI and a working decoy |
| `medium` | one strong signal, optionally corroborated by the path finding                    |
| `low`    | a single weak signal, such as timeouts alone, or a passing target                 |

A timeout-only case is never reported with high confidence.

## Clean targets

A target whose ladder passed is reported as `no blockage detected`. The taxonomy
type stays `unknown` because the taxonomy only describes blockage types; a
separate `no_blockage` flag marks the clean outcome, and the UI shows the human
wording instead of `unknown`.

## Caveats

Every hypothesis carries:

- one or more evidence lines, otherwise it cannot be constructed;
- the rejected alternatives with the reason each was dismissed;
- the single-vantage-point caveat, stating that absence of evidence is not proof
  of blocking.

`unknown` is a valid terminal outcome and is preferred over a weak claim.

<!-- refs -->
