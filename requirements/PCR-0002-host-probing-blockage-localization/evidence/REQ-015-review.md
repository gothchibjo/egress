# REQ-015 documentation review

Date: 2026-11-19 Reviewer sign-off: pending

Document: docs/tool-survey.md

## Checklist

- All surveyed tools are named with their relevant capability and privilege
  model: ping, traceroute, mtr, tcptraceroute, nexttrace, trippy, dig/dog, curl,
  OONI Probe, RIPE Atlas.
- Adopted features map to BR-01 through BR-16; every row in the adopted table
  has a reference link.
- Deferred and rejected features state a reason.
- The document is English.
- No tool outside the surveyed list is named.
- Maintenance section routes scope changes to a new PCR.

## Link check

```text
$ grep -o "^\[BR-[0-9][0-9]\]" docs/tool-survey.md | wc -l
      16
```

## Scope check

```text
$ grep -niE "nmap|hping|nping|tcpdump|wireshark|iperf|smokeping|httping|nslookup|kdig" docs/tool-survey.md
```
