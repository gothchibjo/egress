---
type: reference
author: gothchibjo
date: 2026-11-19
---

# Tool survey and feature rationale <!-- omit in toc -->

- [Scope](#scope)
- [Survey](#survey)
- [Adopted features](#adopted-features)
- [Deferred features](#deferred-features)
- [Rejected features](#rejected-features)
- [Maintenance](#maintenance)

## Scope

`egress` reuses the measurement techniques of established diagnostic tools
instead of inventing new probes. This survey records what each tool does well,
how it obtains privileges, and which capabilities were adopted, deferred or
rejected for v1. It implements [BR-15] and is the rationale behind the scope in
[the business requirements].

The survey covers ping, traceroute, mtr, tcptraceroute, nexttrace, trippy,
dig/dog, curl, OONI Probe and RIPE Atlas. "Path view" means per-hop discovery;
"interpretation" means a verdict that goes beyond raw observations.

## Survey

| Tool          | Relevant capability                                                             | Privilege model                                                                  | Path view | Interpretation |
| :------------ | :------------------------------------------------------------------------------ | :------------------------------------------------------------------------------- | :-------: | :------------- |
| ping          | ICMP echo round-trip time and loss                                              | Unprivileged ICMP datagram socket on modern systems; raw socket historically     |    no     | none           |
| traceroute    | TTL-limited UDP, ICMP or TCP probes with per-hop addresses and round-trip times | Setuid root or raw sockets for ICMP; UDP mode can run unprivileged               |    yes    | none           |
| mtr           | Continuous traceroute plus ping statistics per hop                              | Raw sockets; setuid root or the equivalent capability                            |    yes    | none           |
| tcptraceroute | TTL-limited TCP SYN probes to a chosen port                                     | Raw sockets; normally root                                                       |    yes    | none           |
| nexttrace     | Multi-protocol traceroute with optional ASN and geolocation enrichment          | Raw sockets for ICMP/TCP modes; some modes unprivileged; enrichment fetches data |    yes    | partial        |
| trippy        | Interactive traceroute and ping with charts and per-hop statistics              | Raw sockets; setuid root or the equivalent capability                            |    yes    | none           |
| dig/dog       | DNS record queries; dog also supports encrypted DNS transports                  | Unprivileged                                                                     |    no     | none           |
| curl          | HTTP and HTTPS requests with connection, TLS and proxy diagnostics              | Unprivileged                                                                     |    no     | none           |
| OONI Probe    | Censorship test suites for web connectivity and messaging, with result upload   | Unprivileged; measurements are uploaded to a public platform                     |    no     | partial        |
| RIPE Atlas    | Distributed measurements executed by hosted probes and scheduled through an API | Account and credits; probes run on remote infrastructure                         |    yes    | none           |

The surveyed tools split into four groups: single-probe utilities (ping,
dig/dog, curl), path discovery (traceroute, tcptraceroute, mtr, nexttrace,
trippy), distributed measurement (OONI Probe, RIPE Atlas) and interpretation
layers built on top of those measurements. None combines a transport ladder, a
path table and a blockage verdict for one target from one unprivileged session.

## Adopted features

| Feature                                                | Inspiration                          | BR      | Why adopted                                                             |
| :----------------------------------------------------- | :----------------------------------- | :------ | :---------------------------------------------------------------------- |
| Curated preset catalog with a version                  | OONI test lists, RIPE target lists   | [BR-07] | Common targets without typing hosts; static and reviewable              |
| Multi-select picker with filter and custom targets     | traceroute arguments, dig names      | [BR-01] | One run covers several destinations; presets stay discoverable          |
| Custom host validation with port and protocol override | tcptraceroute port choice, curl URLs | [BR-02] | The admin can test the exact endpoint that fails                        |
| Layered ladder: DNS, TCP, TLS, HTTP, ICMP, trace       | ping, dig, curl, traceroute          | [BR-03] | One target gets one ordered body of evidence instead of six commands    |
| Per-hop path table with drop localization              | traceroute, mtr                      | [BR-04] | The localizing question is where replies stop, not only whether they do |
| Blockage taxonomy and ranked hypothesis                | none of the surveyed tools           | [BR-05] | The admin needs a named failure type, not a wall of timings             |
| Active versus passive evidence labelling               | none of the surveyed tools           | [BR-06] | Silence and active rejection are different observations                 |
| Bounded concurrency with cancellation                  | mtr continuous mode, traceroute      | [BR-08] | Several targets without runaway probes, and an interruptible run        |
| Live results view with target tabs and verdict         | mtr, trippy interactive views        | [BR-09] | The run stays observable while it is still collecting evidence          |
| Schema-validated session export                        | OONI result format, RIPE measurement | [BR-10] | Evidence leaves the terminal reproducibly without an upload             |
| Unprivileged by default with an explicit elevated mode | ping unprivileged sockets, mtr root  | [BR-11] | No setuid requirement and no implicit privilege escalation              |
| Declared destinations and local-only operation         | RIPE Atlas scope, OONI upload model  | [BR-12] | The user can audit every endpoint the tool may contact                  |
| Read-only proxy and VPN context                        | curl proxy diagnostics               | [BR-13] | A tunnel explains path differences without changing the probes          |
| Mandatory limitations and confidence disclosure        | none of the surveyed tools           | [BR-14] | A verdict states its vantage point, false positives and next step       |
| In-app help for picker, ladder, hops and taxonomy      | trippy help, mtr interactive keys    | [BR-16] | The classification is only useful when the terms are explained in place |

## Deferred features

| Feature                                      | Inspiration        | Reason                                                                                 |
| :------------------------------------------- | :----------------- | :------------------------------------------------------------------------------------- |
| Reverse DNS names per hop                    | traceroute         | Adds a DNS query per hop and a new resolver dependency; addresses are enough for v1    |
| Tracing every address of a multi-answer name | dig, traceroute    | Multiplies traffic and run time; v1 traces the primary resolved address                |
| IPv6 traceroute                              | traceroute, ping   | IPv4 covers the v1 catalog; IPv6 needs a separate probe path and validation            |
| Continuous monitoring mode                   | mtr, trippy        | v1 is a one-shot diagnostic session; refresh intervals and history are out of scope    |
| Selectable probe protocol and port per hop   | tcptraceroute      | The ladder fixes the protocol per step; a raw per-hop selector is a later extension    |
| Retry and pacing controls for silent hops    | traceroute options | v1 uses bounded windows; manual tuning adds keys and budget rules without new evidence |
| ASN and geolocation labels per hop           | nexttrace          | Requires an external database or API, which conflicts with local-only operation        |
| Packet capture of the probe traffic          | none of the tools  | Adds a capture dependency and inspection surface that v1 does not need                 |
| Distributed vantage points                   | OONI Probe, RIPE   | Out of scope for a local tool; the export is the hand-off to external validation       |

## Rejected features

| Feature                                    | Inspiration       | Reason                                                                                                     |
| :----------------------------------------- | :---------------- | :--------------------------------------------------------------------------------------------------------- |
| Telemetry and result upload                | OONI Probe        | Violates the local-only rule and the user's expectation that results stay on the machine                   |
| Enrichment through a remote API            | nexttrace         | Adds an undeclared destination and a privacy surface; conflicts with [BR-12]                               |
| Setuid or implicit privilege escalation    | traceroute, mtr   | The tool must run unprivileged by default and never prompt for elevation; [BR-11] keeps elevation explicit |
| Continuous background probing              | mtr               | The tool is a diagnostic session, not a monitoring daemon; background traffic is not declared              |
| Automated remediation or bypass            | none of the tools | The vision is observation only; changing routes, proxies or firewall state is out of scope                 |
| Credentialed proxy configuration           | curl              | Context is read-only; the tool never changes proxy or VPN settings                                         |
| Global IP database bundled with the binary | nexttrace         | Large, stale without updates and not needed for a clear hop table                                          |
| Crowdsourced measurement scheduling        | RIPE Atlas        | Requires accounts, credits and remote probes; v1 stays on the local host                                   |

## Maintenance

This document is part of the frozen scope of PCR-0002. A scope change that adds,
removes or moves a feature between the adopted, deferred and rejected lists is a
new PCR, not an edit here. The survey is reviewed when the catalog, ladder or
export format changes so the rationale keeps matching the implementation.

<!-- refs -->

[BR-01]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-01--target-picker-and-selection
[BR-02]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-02--custom-target-validation
[BR-03]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-03--layered-probe-ladder
[BR-04]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-04--drop-hop-localization
[BR-05]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-05--blockage-type-classification
[BR-06]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-06--probe-interpretation-integrity
[BR-07]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-07--preset-catalog
[BR-08]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-08--scheduling-and-concurrency
[BR-09]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-09--live-results-ui
[BR-10]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-10--json-session-export
[BR-11]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-11--privilege-handling
[BR-12]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-12--local-only-operation-and-probe-transparency
[BR-13]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-13--proxy-and-vpn-context
[BR-14]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-14--limitations-disclosure
[BR-15]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-15--tool-survey-and-feature-rationale
[BR-16]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md#br-16--in-app-help
[the business requirements]:
  ../requirements/PCR-0002-host-probing-blockage-localization/02-business.md
