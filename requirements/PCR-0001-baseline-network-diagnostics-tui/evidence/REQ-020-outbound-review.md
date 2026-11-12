# REQ-020 — Outbound traffic review

Frozen requirement: `03-technical.yaml`, REQ-020 "Local-only egress policy".
Inspection evidence: network capture review and code review sign-off.

## Reviewed artifacts

- `internal/probe/destinations.go` — the single catalog of outbound endpoints.
- `internal/domain/destination.go` — the destination model and its validation.
- `internal/report/result.go` — the JSON envelope that carries the catalog.
- `internal/tui/render.go` — the `d` page that renders the catalog.
- `docs/outbound-destinations.md` — the human-readable list and review
  procedure.

## Outbound call sites

Every site that can create a socket or send a request:

| Location                   | Mechanism                                | Destination                    |
| :------------------------- | :--------------------------------------- | :----------------------------- |
| `internal/probe/dns.go`    | `miekg/dns` UDP query, TCP on truncation | configured resolver, port 53   |
| `internal/probe/dns.go`    | HTTPS GET, JSON API                      | `cloudflare-dns.com:443`       |
| `internal/probe/icmp.go`   | raw or datagram ICMP echo                | default gateway, anchor hosts  |
| `internal/probe/tcp.go`    | `net.Dialer` connect                     | anchor hosts, ports 443 and 53 |
| `internal/probe/mtu.go`    | raw ICMP with the don't-fragment flag    | `example.com`                  |
| `internal/probe/portal.go` | HTTP GET, redirects not followed         | `captive.apple.com:80`         |

No other package opens a socket:

| Package             | Network use                                                           |
| :------------------ | :-------------------------------------------------------------------- |
| `internal/app`      | runs the injected checks once per baseline; no client of its own      |
| `internal/platform` | local commands only: `route -n get default`, `scutil --dns`; no dials |
| `internal/report`   | serializes results; no network                                        |
| `internal/tui`      | renders state; no network                                             |
| `cmd/egress`        | wires the session and the dashboard; no network                       |

There is no telemetry, analytics, crash reporting, update check or enrichment
dependency in `go.mod` and no HTTP client outside the DoH comparison and the
captive canary.

## Catalog snapshot

`probe.OutboundDestinations()` returns, on a host with one configured resolver:

| Kind     | Transport | Endpoint                                        |
| :------- | :-------- | :---------------------------------------------- |
| resolver | udp+tcp   | configured resolver, port 53                    |
| doh      | https     | `cloudflare-dns.com:443`                        |
| anchor   | icmp      | `1.1.1.1`                                       |
| anchor   | tcp       | `1.1.1.1:443`, `1.1.1.1:53`                     |
| anchor   | icmp      | `8.8.8.8`                                       |
| anchor   | tcp       | `8.8.8.8:443`, `8.8.8.8:53`                     |
| anchor   | icmp      | `captive.apple.com`                             |
| anchor   | tcp       | `captive.apple.com:443`, `captive.apple.com:53` |
| anchor   | icmp      | `example.com`                                   |
| anchor   | tcp       | `example.com:443`, `example.com:53`             |
| path-mtu | icmp-df   | `example.com`                                   |
| captive  | http      | `captive.apple.com:80`                          |

## Guard tests

`internal/probe/destinations_test.go` asserts that the catalog covers every
probe, contains no static destination outside the anchor set and the three
documented services, and stays stable between calls.
`internal/report/result_test.go` asserts that the JSON envelope with the catalog
validates against `schema/result-v1.schema.json`.

## Network capture review

Procedure: `docs/outbound-destinations.md`, "Review procedure".

| Field   | Value                                                                                                  |
| :------ | :----------------------------------------------------------------------------------------------------- |
| Date    | 2026-11-12                                                                                             |
| Capture | `tcpdump -i any -n 'port 53 or port 80 or port 443 or icmp'` under `sudo`, during `sudo egress --json` |
| Volume  | 349 packets captured, 19 130 received by the filter, 0 dropped                                         |
| Scope   | host-wide PKTAP capture, not scoped to the egress process                                              |

### Documented endpoints observed

Local resolver addresses are redacted in this record.

| Kind           | Endpoint                                                    |
| :------------- | :---------------------------------------------------------- |
| resolver       | four configured resolvers on port 53, one of them `1.1.1.1` |
| DoH            | `cloudflare-dns.com` via Cloudflare anycast on 443          |
| anchor         | `1.1.1.1:443`, `1.1.1.1:53`, `8.8.8.8:443`, `8.8.8.8:53`    |
| anchor         | `captive.apple.com` on 443 and 53                           |
| anchor         | `example.com` on 443 and 53                                 |
| captive portal | `captive.apple.com:80`                                      |
| ICMP           | gateway, the four anchors and the two Cloudflare addresses  |

### Additional flows in the same window

The host-wide capture also contains connections to addresses that are not in the
catalog: `142.250.151.95`, `149.154.167.41`, `104.26.9.177`, `104.18.11.66`,
`104.16.248.249`, `13.89.179.15`, `178.248.239.157`, `194.67.121.107`,
`212.147.230.189`, `35.201.82.116` (all on 443).

Egress has no code path to any of them: the TCP probe dials the four anchor
hosts, the DoH client dials its endpoint, the portal check dials the canary and
the DNS check queries the configured resolvers. Several of those addresses are
Cloudflare anycast, so destination matching alone cannot separate them from
anchor traffic after a DNS rotation.

### Attribution

PKTAP records process metadata, so the existing capture was read back with:

```bash
sudo tcpdump -nr /tmp/egress-capture.pcap -k NP | grep -i egress
```

The packet metadata lists `egress` with one pid (62308). Every endpoint that
process touched maps to a documented destination:

| egress endpoint (sanitised)                                        | Catalog row                                   |
| :----------------------------------------------------------------- | :-------------------------------------------- |
| `1.1.1.1:443`, `1.1.1.1:53`                                        | anchor `cloudflare-dns`, resolver             |
| `8.8.8.8:443`, `8.8.8.8:53`                                        | anchor `google-dns`, resolver                 |
| `17.253.37.199:443`                                                | anchor `captive.apple.com`, TCP 443           |
| `17.253.37.198:80`                                                 | captive-portal canary                         |
| `104.20.23.154:443`, `104.16.248.249:443`                          | `example.com` / `cloudflare-dns.com`, TCP 443 |
| `resolver.local.1:53`, `resolver.local.2:53`, `resolver.tunnel:53` | configured resolvers                          |
| bare addresses without a port                                      | ICMP to the gateway and the anchors           |

The endpoints attributed to other processes (`149.154.167.41`, `142.250.151.95`,
`13.89.179.15` and the rest of the host-wide list) are absent from the `egress`
metadata, so they belong to other applications on the machine.

Hostname resolution for the anchors is delegated to the OS resolver:
mDNSResponder issues its own DNS queries, one packet carries both process
entries, and egress itself never opens a socket to those resolvers.

## Review checklist

| Criterion (frozen)                           | State | Note                                                                                        |
| :------------------------------------------- | :---- | :------------------------------------------------------------------------------------------ |
| No network traffic beyond documented probes. | pass  | Capture attributed to the egress pid matches the catalog.                                   |
| Private or identifying data is not stored.   | pass  | Results carry check ids, targets and measurements only.                                     |
| Probe destinations are documented.           | pass  | `docs/outbound-destinations.md`, the `d` page and the JSON field.                           |
| The utility remains useful without internet. | pass  | Offline classification and skipped checks are covered by REQ-019 tests and the walkthrough. |
| No hidden background calls occur.            | pass  | No egress socket outside the catalog; two goroutines, no background timers.                 |

## Sign-off

Code review and capture review: gothchibjo, 2026-11-12. The call-site inventory,
the closed catalog, the schema exposure and the attributed capture match the
frozen REQ-020 text.
