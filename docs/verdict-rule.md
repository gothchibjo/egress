---
type: reference
author: gothchibjo
date: 2026-11-12
---

# Aggregate verdict rule <!-- omit in toc -->

- [Scope](#scope)
- [Input and evidence guard](#input-and-evidence-guard)
- [Anchor scoping](#anchor-scoping)
- [Effective severity](#effective-severity)
- [Overall level](#overall-level)
- [Most actionable finding](#most-actionable-finding)
- [Determinism](#determinism)
- [Examples](#examples)

## Scope

This rule implements the overall verdict required by [REQ-018] and [BR-04]. It
is global: statuses are aggregated across every check regardless of path layer.
Layer grouping is a dashboard concern, described by [REQ-025], and does not
change the verdict.

Implementation: `internal/domain/verdict.go`, entry point `AggregateVerdict`.

## Input and evidence guard

The input is the ordered list of results from one completed baseline, in
baseline order.

| Condition                                   | Outcome                              |
| :------------------------------------------ | :----------------------------------- |
| No results                                  | no verdict, `ErrNoResults`           |
| Any result fails `Result.Validate`          | no verdict, `ErrInvalidVerdictInput` |
| Every result carries its evidence or reason | a verdict is computed                |

A result that cannot explain its own status never contributes to a verdict. The
evidence rules are defined by [REQ-001].

## Anchor scoping

A result is _anchor-scoped_ when its target host equals a curated anchor host
from `internal/domain/anchor.go`; scheme, port, path, user info, a trailing dot
and letter case are ignored. A target such as `example.com:443` or
`https://captive.apple.com/hotspot-detect.html` therefore matches its anchor.
Subdomains do not match: `www.example.com` is not the `example.com` anchor.

Every other result is _core_.

## Effective severity

| Result                     | Effective severity                                    |
| :------------------------- | :---------------------------------------------------- |
| `fail` on a core target    | fail                                                  |
| `fail` on an anchor target | warn, unless the anchor-failure condition below holds |
| `warn` on any target       | warn                                                  |
| `pass`                     | none                                                  |
| `skipped`                  | none                                                  |

Anchor-failure condition: anchor failures keep full severity only when every
anchor-scoped result failed and at least two distinct curated anchors were
checked. A path-wide failure across vendors is a blockage; a single failing
anchor, or the only anchor checked failing, is a warning.

## Overall level

| Condition                                              | Level      |
| :----------------------------------------------------- | :--------- |
| At least one result has effective severity fail        | `blocked`  |
| At least one result has effective severity warn        | `degraded` |
| No fail or warn, at least one `pass`                   | `healthy`  |
| No fail or warn, no `pass` (every result is `skipped`) | `degraded` |

Status counts are the raw number of results per status. They describe the run;
the level applies the rule above to the same results.

## Most actionable finding

The finding is selected in this order, with the first match in baseline order:

1. the first result with effective severity fail;
2. the first anchor failure downgraded to warn;
3. the first `warn` result;
4. the first `skipped` result, only when the level is `degraded` and no result
   passed;
5. otherwise none, which is the healthy case.

A finding is a copy of the result, so callers cannot mutate the run through it.

## Determinism

The rule makes one forward pass over the input, uses a fixed status precedence
(`fail` > `warn` > `skipped` > `pass`) and breaks ties by baseline order. Map
iteration only accumulates counts and the set of checked anchors; it never
selects the level or the finding. Identical results in identical order always
produce an identical verdict.

## Examples

| Results in baseline order                     | Level      | Finding              |
| :-------------------------------------------- | :--------- | :------------------- |
| gateway pass, DNS pass                        | `healthy`  | none                 |
| gateway pass, HTTP warn                       | `degraded` | HTTP warn            |
| gateway fail, DNS pass                        | `blocked`  | gateway fail         |
| ICMP to `1.1.1.1` fail, TCP to `8.8.8.8` pass | `degraded` | ICMP anchor fail     |
| ICMP to `1.1.1.1` fail only                   | `degraded` | ICMP anchor fail     |
| Cloudflare, Google and Apple anchors all fail | `blocked`  | first anchor fail    |
| ICMP skipped, TCP pass                        | `healthy`  | none                 |
| ICMP skipped, TCP skipped                     | `degraded` | first skipped result |

<!-- refs -->

[REQ-018]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
[BR-04]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/02-business.md
[REQ-025]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
[REQ-001]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
