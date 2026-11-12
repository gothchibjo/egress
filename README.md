# egress <!-- omit in toc -->

`egress` is a terminal UI that diagnoses network blockages. It runs a baseline
health check at startup, then probes curated anchors to localize the hop where
traffic stops and classifies the failure as local link, gateway, DNS or
upstream.

- [Requirements](#requirements)
- [Install](#install)
- [Run](#run)
- [Privilege modes](#privilege-modes)
- [Privacy](#privacy)
- [JSON output](#json-output)
- [Exit codes](#exit-codes)
- [Keys](#keys)
- [Documentation](#documentation)
- [Development](#development)
- [License](#license)

## Requirements

- macOS or Linux. Windows is not supported in v1.
- Go 1.27 or newer to build from source.

## Install

```bash
go install github.com/gothchibjo/egress/cmd/egress@latest
```

Or build from a checkout:

```bash
go build ./cmd/egress
```

## Run

```bash
egress
```

The dashboard starts the baseline immediately, shows the running check in the
footer and reports the results when it finishes. Press `?` for the key map and
`q` to quit.

## Privilege modes

| Mode           | How to start                                 | Checks                                                                        |
| :------------- | :------------------------------------------- | :---------------------------------------------------------------------------- |
| `unprivileged` | `egress`                                     | every check that needs no raw socket; raw-only checks are skipped with a hint |
| `elevated`     | start the binary as root, e.g. `sudo egress` | adds the raw-socket checks, including the path-MTU probe                      |

`egress` never prompts for privileges, never re-execs itself with `sudo` and
never changes host settings. The dashboard labels the session mode, and the JSON
envelope carries the same label in `mode`.

## Privacy

`egress` is local-only. It sends no telemetry, analytics, crash reports, result
uploads or enrichment requests. The only outbound traffic is the documented
probe set in [the outbound destinations]; the same list is rendered on the `d`
page and included in the JSON `destinations` field. Proxy values are reported
with credentials redacted.

## JSON output

```bash
egress --json     # one JSON document on stdout
egress --schema   # print the JSON schema
```

When stdout is not a terminal, `egress` runs non-interactively and writes JSON
without waiting for `--json`, so `egress | jq` works. The document carries the
results plus the verdict, the failure classification when present, the proxy and
tunnel context and the destination list. The schema is [the JSON schema]; the
envelope and the field semantics are described in [the JSON output reference].

## Exit codes

| Code | Meaning                                                    |
| ---: | :--------------------------------------------------------- |
|    0 | healthy: every check passed or was skipped                 |
|    1 | degraded: warnings, an isolated anchor failure, or no pass |
|    2 | blocked: a core check failed or every anchor failed        |
|    3 | usage or runtime error, including an invalid command line  |

The codes follow [the verdict rule] and apply to the dashboard and to the JSON
output.

## Keys

Press `?` inside the dashboard for the full key map and the verdict semantics.
The same map is in [the key map]; the dashboard is operable without a mouse.

## Documentation

| Document                  | Content                                            |
| :------------------------ | :------------------------------------------------- |
| [the anchor list]         | the curated multi-vendor anchors and their purpose |
| [the verdict rule]        | how the aggregate verdict is derived               |
| [the JSON schema]         | the stable envelope validated by the test suite    |
| [the key map]             | every key and page                                 |
| [the requirements record] | the frozen specification and its evidence          |

## Development

```bash
go test -race ./...
go vet ./...
go build ./...
```

Unit tests never require external network access. Network-dependent checks are
integration tests and are opt-in through `EGRESS_INTEGRATION=1`.

## License

MIT. See [LICENSE].

<!-- refs -->

[the outbound destinations]: docs/outbound-destinations.md
[the JSON output reference]: docs/json-output.md
[the JSON schema]: internal/report/schema/result-v1.schema.json
[the verdict rule]: docs/verdict-rule.md
[the key map]: docs/key-map.md
[the anchor list]:
  requirements/PCR-0001-baseline-network-diagnostics-tui/evidence/REQ-003-anchor-review.md
[the requirements record]:
  requirements/PCR-0001-baseline-network-diagnostics-tui/
[LICENSE]: LICENSE
