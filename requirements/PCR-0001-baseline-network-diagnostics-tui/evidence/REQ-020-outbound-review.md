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

State: not yet captured. The run requires an elevated `tcpdump` on the review
machine; the capture and the resulting endpoint list are recorded here when the
walkthrough runs.

## Review checklist

| Criterion (frozen)                           | State   | Note                                                                       |
| :------------------------------------------- | :------ | :------------------------------------------------------------------------- |
| No network traffic beyond documented probes. | pending | Call sites are inventoried; the capture has not run yet.                   |
| Private or identifying data is not stored.   | pass    | Results carry check ids, targets and measurements only.                    |
| Probe destinations are documented.           | pass    | `docs/outbound-destinations.md`, the `d` page and the JSON field.          |
| The utility remains useful without internet. | partial | Local and skipped paths exist; the offline walkthrough belongs to REQ-019. |
| No hidden background calls occur.            | pass    | Two goroutines exist: the baseline run and the dashboard command loop.     |

## Sign-off

Pending review by gothchibjo. The requirement stays `open` until the capture is
recorded and this sign-off is filled in.
