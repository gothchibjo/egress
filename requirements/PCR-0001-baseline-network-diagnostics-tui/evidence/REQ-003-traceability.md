# REQ-003 — Traceability matrix

| Layer          | Reference                                                             |
| :------------- | :-------------------------------------------------------------------- |
| Business rule  | BR-10 — Curated multi-vendor anchors                                  |
| Requirement    | REQ-003 — Curated multi-vendor anchor set (must, xs)                  |
| Implementation | `internal/domain/anchor.go` — `Anchor`, `BaselineAnchors()`           |
| Test           | `internal/domain/anchor_test.go` — catalog, uniqueness, vendors, copy |
| Test report    | `evidence/REQ-003-anchor-tests.txt`                                   |
| Review         | `evidence/REQ-003-anchor-review.md`                                   |
| Guard          | anchor set remains multi-vendor — pass                                |
| Guard          | no single vendor determines the verdict alone — delegated to REQ-018  |

## Requirement to implementation

| REQ-003 element                  | Implementation                                |
| :------------------------------- | :-------------------------------------------- |
| Cloudflare `1.1.1.1` anchor      | `BaselineAnchors()` entry `cloudflare-dns`    |
| Google `8.8.8.8` anchor          | `BaselineAnchors()` entry `google-dns`        |
| Apple captive-portal canary      | `BaselineAnchors()` entry `apple-captive`     |
| `example.com` anchor             | `BaselineAnchors()` entry `example-com`       |
| Report each anchor independently | one `Anchor` value per destination, stable id |

## Open evidence

| Missing artifact                          | Owner requirement |
| :---------------------------------------- | :---------------- |
| TUI and JSON anchor visibility            | REQ-025, REQ-022  |
| Warning verdict for one failing anchor    | REQ-018           |
| Reviewer sign-off on the destination list | `gothchibjo`      |
