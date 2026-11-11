# REQ-003 — Anchor list review

Frozen requirement: `03-technical.yaml`, REQ-003 "Curated multi-vendor anchor
set". Inspection evidence: anchor list review sign-off.

## Reviewed artifact

`internal/domain/anchor.go` — `BaselineAnchors()` returns a defensive copy of
the catalog; `internal/domain/anchor_test.go` checks completeness, unique ids,
multi-vendor coverage and immutability.

## Anchor list

| ID               | Vendor     | Host                | Purpose                                 |
| :--------------- | :--------- | :------------------ | :-------------------------------------- |
| `cloudflare-dns` | Cloudflare | `1.1.1.1`           | public resolver and reachability anchor |
| `google-dns`     | Google     | `8.8.8.8`           | public resolver and reachability anchor |
| `apple-captive`  | Apple      | `captive.apple.com` | captive-portal HTTP canary              |
| `example-com`    | IANA       | `example.com`       | neutral DNS and HTTP anchor             |

Four vendors, one anchor each: Cloudflare, Google, Apple, IANA.

## Review checklist

| Criterion (frozen)                                            | State   | Note                                                             |
| :------------------------------------------------------------ | :------ | :--------------------------------------------------------------- |
| All four anchors are present in every baseline run.           | partial | Catalog is complete; run wiring arrives with the probe set.      |
| Each anchor result is visible separately in the TUI and JSON. | pending | Needs the dashboard adapter (REQ-025) and JSON output (REQ-022). |
| One failing anchor while others pass yields a warning.        | pending | Verdict rule is REQ-018; anchor isolation is required there.     |
| The anchor list is documented and consistent in TUI and JSON. | partial | Documented here and in `anchor.go`; consumers do not exist yet.  |

## Sign-off

Reviewer: pending. The list matches BR-10 and the frozen REQ-003 text; a
sign-off by `gothchibjo` records approval of the destinations themselves. The
requirement stays `open` until the TUI and JSON criteria are verifiable.
