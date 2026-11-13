# REQ-001 — Catalog review

Frozen requirement: `03-technical.yaml`, REQ-001 "Static preset target catalog".
Inspection evidence: catalog review sign-off and traceability matrix.

## Reviewed artifacts

- `internal/domain/target.go` — `PresetCatalog()` and `CatalogVersion`.
- `internal/domain/target_test.go` — content, copy, determinism and version
  tests.
- `docs/target-catalog.md` — the documented catalog.

## Catalog

| ID               | Display name   | Host             | Category       | Ports   |
| :--------------- | :------------- | :--------------- | :------------- | :------ |
| `youtube`        | YouTube        | `youtube.com`    | video          | 443     |
| `telegram`       | Telegram       | `telegram.org`   | messaging      | 443     |
| `discord`        | Discord        | `discord.com`    | messaging      | 443     |
| `github`         | GitHub         | `github.com`     | developer      | 443     |
| `google`         | Google         | `google.com`     | search         | 443     |
| `cloudflare`     | Cloudflare     | `cloudflare.com` | infrastructure | 443     |
| `wikipedia`      | Wikipedia      | `wikipedia.org`  | reference      | 443     |
| `x`              | X              | `x.com`          | social         | 443     |
| `tor`            | Tor            | `torproject.org` | anonymity      | 443     |
| `cloudflare-dns` | Cloudflare DNS | `1.1.1.1`        | infrastructure | 443, 53 |
| `google-dns`     | Google DNS     | `8.8.8.8`        | infrastructure | 443, 53 |

## Checklist

| Criterion (frozen)                                                  | State   | Where                                                    |
| :------------------------------------------------------------------ | :------ | :------------------------------------------------------- |
| All eleven entries are present and selectable by display name.      | pass    | `TestPresetCatalogContents`; the picker lands in REQ-002 |
| Each entry exposes the address that will be probed.                 | pass    | `Host` is the probe address; tests reject URLs           |
| The catalog loads with no configuration file and no network access. | pass    | constant slice, accessor performs no I/O                 |
| JSON export identifies the catalog version.                         | pending | the session export arrives with REQ-011                  |
| No remote catalog update is attempted in v1.                        | pass    | no update client, URL or dependency                      |

## Notes

- `PresetCatalog()` returns deep copies; tests mutate the result and re-read the
  catalog.
- The order is fixed and deterministic, so the picker and export are stable.
- Category labels are lowercase product categories, not a taxonomy of blockages;
  they do not overlap with REQ-007.

## Sign-off

Pending review by gothchibjo. The requirement stays `open` until the session
export carries the catalog version (REQ-011).
