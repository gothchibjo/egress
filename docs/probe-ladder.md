---
type: reference
author: gothchibjo
date: 2026-11-13
---

# Probe ladder <!-- omit in toc -->

- [Steps](#steps)
- [Timing](#timing)
- [Independence](#independence)
- [Destinations](#destinations)

## Steps

The ladder runs per target in this order. Every step is a separate result with
its own status, measurements, timestamp and duration.

| Order | Step         | What it does                                                                                                                                     |
| :---: | :----------- | :----------------------------------------------------------------------------------------------------------------------------------------------- |
|   1   | `dns`        | resolves the hostname with the system resolver and compares the answer with the public DoH endpoint; an IP literal skips this step with a reason |
|   2   | `tcp.<port>` | one TCP connect per target port, classified as connected, refused, reset or timeout                                                              |
|   3   | `tls.real`   | TLS ClientHello with the target host as SNI                                                                                                      |
|   4   | `tls.decoy`  | the same ClientHello with `example.org` as SNI                                                                                                   |
|   5   | `tls.absent` | the same ClientHello without SNI                                                                                                                 |
|   6   | `http`       | one HTTP GET on the primary port (`https` unless the target is http-only)                                                                        |
|   7   | `icmp`       | one ICMP echo to the resolved address                                                                                                            |
|   8   | `trace`      | traceroute; reported as skipped until the tracing method lands with REQ-005                                                                      |

The three SNI variants stay distinguishable by step id and by the `sni`
measurement. A TLS alert is a warning, not a failure; a network error is a
failure.

### Tracing methods

| Session                  | Method                                                              | Requirement                                                                               |
| :----------------------- | :------------------------------------------------------------------ | :---------------------------------------------------------------------------------------- |
| `limited` (unprivileged) | UDP probes with growing TTL, ICMP errors read through a ping socket | works without root                                                                        |
| `elevated`               | ICMP echo probes with growing TTL over a raw socket                 | needs a raw socket; falls back to UDP with a stated reason when the socket is unavailable |

## Timing

Every step runs under a 2 s context deadline. A step that times out records the
outcome as `fail` and does not affect the schedule of the following steps.

## Independence

A failing or skipped step never cancels a later step. A DNS failure still allows
TCP and TLS probes when the host is an IP literal, and a TCP refusal still lets
the HTTP and ICMP steps run.

## Localization

The hop table feeds `domain.LocalizePath`, which yields one of three findings:

- `path complete at hop N` when the target answered;
- `drop after hop N` when the replies stop and the ladder did not reach the
  target;
- `inconclusive between hops N and M` when the trace was truncated or the target
  answered the ladder, so the silence is likely ICMP filtering.

Every finding lists the alternative explanations; a silent hop alone is never
presented as proof of blocking. The localization is cross-checked with the
ladder's TCP, DNS and HTTP outcomes.

## Destinations

The ladder dials only the selected target and the configured resolvers plus the
documented DoH endpoint. There is no background probe and no enrichment call.

<!-- refs -->
