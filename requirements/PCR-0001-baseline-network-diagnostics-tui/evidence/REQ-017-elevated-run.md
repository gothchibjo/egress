# REQ-017 — Elevated mode test report

Frozen requirement: `03-technical.yaml`, REQ-017 "Optional elevated mode". Test
evidence: elevated mode test report and traceability matrix.

## Run

| Field          | Value                                                                                     |
| :------------- | :---------------------------------------------------------------------------------------- |
| Command        | `sudo /tmp/egress --json`                                                                 |
| Date           | 2026-11-12                                                                                |
| Mode           | `elevated`                                                                                |
| Checks         | 17                                                                                        |
| Verdict        | `degraded` (pass 13, warn 2, fail 2, skipped 0)                                           |
| Classification | `upstream` — "Local link, gateway and name resolution were usable; remote probes failed." |

## Difference from the unprivileged run

| Check          | Unprivileged (`REQ-016-unprivileged-run.txt`)       | Elevated                                                                                    |
| :------------- | :-------------------------------------------------- | :------------------------------------------------------------------------------------------ |
| `mtu.pmtu`     | skipped: "raw ICMP DF probe requires elevated mode" | warn: tested 576/1280/1400, max 1280, failed 1400, "suspected PMTU blackhole at 1400 bytes" |
| skipped checks | 1                                                   | 0                                                                                           |

Check ids, order and evidence fields are identical in both modes.

## Observed results

| Check                         | Status | Evidence                            |
| :---------------------------- | :----- | :---------------------------------- |
| `tcp.anchor.apple-captive.53` | fail   | timeout, passive                    |
| `tcp.anchor.example-com.53`   | fail   | timeout, passive                    |
| `icmp.anchor.apple-captive`   | warn   | 33 % loss, ICMP may be rate-limited |
| `mtu.pmtu`                    | warn   | blackhole suspected at 1400 bytes   |
| all other checks              | pass   | 13 passing                          |

## Review notes

- The session mode is carried in the JSON `mode` field and in the dashboard
  header, as required.
- No check changed meaning: the raw-socket checks ran instead of being skipped,
  and the rest produced the same fields.
- The utility did not escalate privileges itself; the operator started it under
  `sudo`.
- Operator observation: `sudo egress --json` prints nothing for several seconds
  while the baseline runs, which can look like a hang. This is outside the
  frozen REQ-022 scope (no progress requirement in non-interactive output) and
  is recorded as a candidate for a future PCR.
