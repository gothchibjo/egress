---
type: reference
author: gothchibjo
date: 2026-11-12
---

# Outbound destinations <!-- omit in toc -->

- [Policy](#policy)
- [Destinations](#destinations)
- [Enforcement](#enforcement)
- [Review procedure](#review-procedure)

## Policy

`egress` is local-only. It sends no telemetry, analytics, crash reports, result
uploads or enrichment requests. The only outbound traffic is the diagnostic
probe set documented here, as required by [REQ-020].

The TUI renders the same list on the `d` page, and the JSON envelope carries it
in the `destinations` field.

## Destinations

| Kind     | Transport | Endpoint                                                 | Purpose                                     |
| :------- | :-------- | :------------------------------------------------------- | :------------------------------------------ |
| resolver | udp+tcp   | each configured nameserver, port 53                      | canary query against the host resolvers     |
| doh      | https     | `cloudflare-dns.com:443`                                 | public DoH comparison for the canary        |
| anchor   | icmp      | `1.1.1.1`, `8.8.8.8`, `captive.apple.com`, `example.com` | ICMP reachability and latency sample        |
| anchor   | tcp       | every anchor on ports 443 and 53                         | TCP reachability to the anchor              |
| path-mtu | icmp-df   | `example.com`                                            | path MTU probe with the don't-fragment flag |
| captive  | http      | `captive.apple.com:80`                                   | captive-portal canary request               |

Notes:

- The configured resolver list comes from the host: `/etc/resolv.conf` on Linux,
  `scutil --dns` on macOS. Queries use UDP and fall back to TCP when a response
  is truncated.
- Anchor hostnames are resolved through the same configured resolvers, so the
  name lookups are covered by the resolver row.
- The captive-portal canary does not follow redirects; a portal response is
  reported, never authenticated to.
- Tracing is not part of the v1 baseline; the probing session adds its own `udp`
  target row for TTL-limited probes.

## Session destinations

A probing session is narrower than the baseline. It may contact only:

| Kind     | Transport      | Endpoint                                     | Purpose                              |
| :------- | :------------- | :------------------------------------------- | :----------------------------------- |
| target   | tcp/https/http | each selected target on its configured ports | TCP, TLS and HTTP ladder steps       |
| target   | icmp           | each selected target                         | ICMP echo step                       |
| target   | udp            | each selected target, high destination ports | TTL-limited trace                    |
| resolver | tcp            | each configured nameserver, port 53          | target name lookups and DoH fallback |
| doh      | https          | `cloudflare-dns.com:443`                     | public DoH comparison                |

The run view shows the list on the `d` page, and the `session-v1` export carries
it in the `destinations` field. Target rows keep the declared host and add the
address the ladder actually probed, so the same name resolving to a CDN edge is
visible in both the UI and the export. No anchor, path-MTU or captive-portal
endpoint is contacted by a probing session.

The session does not create or store a persistent identifier. Export contains
only the current targets, steps, paths, hypotheses and the context described in
[the network context]. With no internet access, each ladder step records its own
failure diagnostic instead of retrying in the background.

## Enforcement

The catalog is built once in `internal/probe/destinations.go` from the same
constants the probes use, so documentation cannot drift from traffic. Guard
tests in `internal/probe/destinations_test.go` assert that:

- every anchor has an ICMP row and a row for each probed TCP port;
- the DoH endpoint, path-MTU host and captive-portal endpoint are present;
- no static destination falls outside the anchor set and the three documented
  services;
- every row passes `domain.Destination.Validate`;
- session destinations are built only from the selected targets, configured
  resolvers and the DoH endpoint, and are covered by
  `internal/domain/session_destination_test.go`.

There is no analytics SDK, crash reporter, update checker or enrichment client
in the module. HTTP is used only by the DoH comparison and the captive canary.

## Review procedure

A network capture review confirms the list against observed traffic:

```bash
# Run the baseline while capturing, then inspect the endpoints contacted.
sudo tcpdump -i any -n 'port 53 or port 80 or port 443 or icmp' -w egress-capture.pcap
sudo egress
sudo tcpdump -nr egress-capture.pcap | awk '{print $3, $5}'
```

Every remote address in the capture must map to a row above. Established sockets
of a running session can be listed without root:

```bash
lsof -nP -i -a -p "$(pgrep -n egress)"
```

The recorded capture and the code review sign-off live in the evidence file for
this requirement.

<!-- refs -->

[the network context]: network-context.md
[REQ-020]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
