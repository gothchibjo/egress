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

## Timing

Every step runs under a 2 s context deadline. A step that times out records the
outcome as `fail` and does not affect the schedule of the following steps.

## Independence

A failing or skipped step never cancels a later step. A DNS failure still allows
TCP and TLS probes when the host is an IP literal, and a TCP refusal still lets
the HTTP and ICMP steps run.

## Destinations

The ladder dials only the selected target and the configured resolvers plus the
documented DoH endpoint. There is no background probe and no enrichment call.

<!-- refs -->
