# PCR-0001 — Requirements traceability

Frozen contract: `03-technical.yaml`. Business requirements: `02-business.md`.
This file maps every REQ to its implementation and its verification evidence; it
is referenced by the REQs whose evidence names a traceability matrix.

| BR           | REQ     | Implementation                                                      | Evidence                                                      |
| :----------- | :------ | :------------------------------------------------------------------ | :------------------------------------------------------------ |
| BR-03        | REQ-001 | `internal/domain/result.go`, `internal/report/result.go`, `schema/` | `REQ-001-unit-tests.txt`, `REQ-001-schema-validation.txt`     |
| BR-01        | REQ-002 | `internal/app/runner.go`, `cmd/egress/main.go`, `internal/tui/`     | `REQ-002-runner-tests.txt`, `REQ-002-tui-lifecycle-tests.txt` |
| BR-10        | REQ-003 | `internal/domain/anchor.go`                                         | `REQ-003-anchor-review.md`, `REQ-003-traceability.md`         |
| BR-02        | REQ-004 | `internal/probe/local.go`, `internal/platform/link.go`              | `REQ-004-local-tests.txt`, `REQ-015-platform-matrix.md`       |
| BR-02        | REQ-005 | `internal/probe/icmp.go` (gateway), `internal/platform/route_*`     | `REQ-005-icmp-tests.txt`                                      |
| BR-02        | REQ-006 | `internal/probe/dns.go`, `internal/platform/dns_config.go`          | `REQ-006-dns-tests.txt`, `probe-integration-live.txt`         |
| BR-02        | REQ-007 | `internal/probe/dns.go` (DoH client)                                | `REQ-007-doh-tests.txt`, `probe-integration-live.txt`         |
| BR-02, BR-15 | REQ-008 | `internal/platform/proxy.go`                                        | `REQ-008-unit-tests.txt`, `REQ-008-traceability.md`           |
| BR-02        | REQ-009 | `internal/probe/icmp.go`, `internal/probe/icmp_client.go`           | `REQ-009-anchor-icmp-tests.txt`, `probe-integration-live.txt` |
| BR-02        | REQ-010 | `internal/probe/tcp.go`                                             | `REQ-010-tcp-tests.txt`, `probe-integration-live.txt`         |
| BR-02, BR-12 | REQ-011 | `internal/probe/portal.go`                                          | `REQ-011-portal-tests.txt`, `probe-integration-live.txt`      |
| BR-02, BR-12 | REQ-012 | `internal/probe/mtu.go`, `mtu_prober.go`, `mtu_df_*`                | `REQ-012-mtu-tests.txt`                                       |
| BR-02        | REQ-013 | `internal/probe/icmp.go` (sample statistics)                        | `REQ-013-latency-tests.txt`, `probe-integration-live.txt`     |
| BR-07        | REQ-014 | `internal/app/runner.go`, `internal/tui/keys.go`                    | `REQ-014-budget-tests.txt`, `REQ-014-cancellation-tests.txt`  |
| BR-08        | REQ-015 | `internal/platform/capabilities*.go`                                | `REQ-015-platform-tests.txt`, `REQ-015-platform-matrix.md`    |
| BR-06        | REQ-016 | `internal/platform/privilege*.go`, probe skip paths                 | `REQ-016-privilege-tests.txt`, `REQ-016-unprivileged-run.txt` |
| BR-13        | REQ-017 | `internal/platform/privilege*.go`, `internal/tui/render.go`         | `REQ-017-elevated-tests.txt`                                  |
| BR-04        | REQ-018 | `internal/domain/verdict.go`, `docs/verdict-rule.md`                | `REQ-018-verdict-tests.txt`                                   |
| BR-11        | REQ-019 | `internal/domain/classification.go`, `internal/tui/render.go`       | `REQ-019-classification-tests.txt`, `REQ-019-tui-tests.txt`   |
| BR-09        | REQ-020 | `internal/probe/destinations.go`, `internal/report/result.go`       | `REQ-020-outbound-review.md`, `docs/outbound-destinations.md` |
| BR-05, BR-16 | REQ-021 | `internal/tui/keys.go`, `docs/key-map.md`                           | `REQ-021-keys-tests.txt`, `REQ-021-walkthrough.txt`           |
| BR-14        | REQ-022 | `cmd/egress/main.go`, `internal/report/`, `docs/json-output.md`     | `REQ-022-json-schema.txt`, `REQ-022-cli-tests.txt`            |
| BR-15        | REQ-023 | `internal/platform/context.go`, `internal/tui/render.go`            | `REQ-023-unit-tests.txt`, `REQ-023-tui-tests.txt`             |
| BR-16        | REQ-024 | `README.md`                                                         | `REQ-024-readme-review.md`                                    |
| BR-04        | REQ-025 | `internal/tui/render.go`, `internal/tui/dashboard.go`               | `REQ-025-layout-tests.txt`, `REQ-025-terminal-matrix.md`      |

## Gaps

- Interactive TUI walkthroughs are pending for REQ-002, REQ-011, REQ-016,
  REQ-019, REQ-021, REQ-023 and REQ-025; the scripted transcript in
  `REQ-021-walkthrough.txt` covers every page but needs a reviewer.
- The elevated run and the network capture require an operator with `sudo`;
  neither could run in this environment.
- Code review sign-offs are recorded in the reviewer's own time: REQ-015,
  REQ-018, REQ-020, REQ-021, REQ-024 and REQ-025.
