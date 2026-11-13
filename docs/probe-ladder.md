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

| Session                  | Method                                                              | Requirement                                                                              |
| :----------------------- | :------------------------------------------------------------------ | :--------------------------------------------------------------------------------------- |
| `limited` (unprivileged) | UDP probes with growing TTL, ICMP errors read through a ping socket | works without root; uses the standard traceroute port layout so the target keeps its TTL |
| `elevated`               | ICMP echo probes with growing TTL over a raw socket                 | needs a raw socket; falls back to UDP with a stated reason when unavailable              |

## Timing

Every step runs under a 2 s context deadline. A step that times out records the
outcome as `fail` and does not affect the schedule of the following steps.

## Independence

A failing or skipped step never cancels a later step. A DNS failure still allows
TCP and TLS probes when the host is an IP literal, and a TCP refusal still lets
the HTTP and ICMP steps run.

## Scheduling

A session probes at most four targets concurrently, and each target's ladder
steps run sequentially. UDP traces are serialized: the standard traceroute port
range is small, so concurrent traces would otherwise read each other's ICMP
errors. `CancelTarget` stops one target; `CancelAll` stops the session. A
cancelled target keeps the steps it collected before the cancellation, so
partial evidence stays exportable.

## Reading the hop table

- `TTL` is the hop number, `ADDRESS` every router that answered the probes at
  that TTL (joined with commas), `LOSS` the share of probes without a reply and
  `RTT` their median round trip. This is the traceroute layout.
- A leading marker flags what deserves attention: `→` is the hop that answered
  as the target, `!` is a hop with partial loss and `x` is the silent hop where
  the replies stop. A blank marker is a normal hop.
- `*` means no reply arrived for that TTL; the router may rate-limit or filter
  the probe. It does not prove the hop is down.
- A path through a tunnel (for example a VPN `utun` interface) often produces no
  ICMP errors at all: the gateway is reachable, but the probes are encapsulated.
- When no hop answers, the pane states that no probe reply arrived and whether
  the target answered a transport step, instead of drawing every TTL as a broken
  hop.
- Trailing silent hops are trimmed. One row is kept when the target was not
  reached, to show where the replies stop; when a transport step reached the
  target, no trailing star is drawn and the summary explains that the remaining
  TTLs returned no ICMP.
- The pane title shows the tracing method and the address that was traced.
- Reverse DNS names are not resolved in v1; the table lists addresses only.

## Localization

The hop table feeds `domain.LocalizePath`, which yields one of three findings:

- `path complete at hop N` when the target answered;
- `drop after hop N` when the replies stop and the ladder did not reach the
  target;
- `inconclusive between hops N and M` when the trace was truncated, or when the
  target answered a transport step but the hops returned no probe reply.

Every finding lists the alternative explanations; a silent hop alone is never
presented as proof of blocking. The localization is cross-checked with the
ladder's TCP, DNS and HTTP outcomes.

## Destinations

The ladder dials only the selected target and the configured resolvers plus the
documented DoH endpoint. There is no background probe and no enrichment call.

<!-- refs -->
