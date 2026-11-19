# REQ-012 network inventory review

Date: 2026-11-19 Reviewer sign-off: pending

## Capture limitation

A root capture with `sudo tcpdump` requires an interactive password on this host
and could not be produced in this session. The review uses a live socket
inventory of the real probe process plus the call-site inventory in
REQ-012-code-review.md.

## Session under review

Three presets (GitHub, Telegram, Discord), limited mode, four rounds. Socket
inventory was sampled every 500 ms for 25 s:

```bash
lsof -nP -i -a -p <probe-pid>
```

## Observed remote endpoints

| Address         | Ports                | Declared as                         |
| :-------------- | :------------------- | :---------------------------------- |
| 140.82.121.4    | 443, 33434-33523/udp | target github.com                   |
| 149.154.167.99  | 443, 33452-33508/udp | target telegram.org                 |
| 162.159.136.232 | 443                  | target discord.com (one DNS answer) |
| 104.16.249.249  | 443                  | DoH endpoint cloudflare-dns.com     |

No other remote address appears in the inventory.

## Unique endpoints

```text
104.16.249.249
140.82.121.4
149.154.167.99
162.159.136.232
```

## Sample

```text
COMMAND     PID USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
egress-ca 57555  max    5u  IPv4 0x847d0f479224dd77      0t0  TCP 100.115.3.197:50272->149.154.167.99:443 (SYN_SENT)
egress-ca 57555  max    9u  IPv4 0xba9e9706c48f8da9      0t0  TCP 100.115.3.197:50268->104.16.249.249:443 (ESTABLISHED)
egress-ca 57555  max   10u  IPv4 0x76d7c310b2af2906      0t0  TCP 100.115.3.197:50271->140.82.121.4:443 (SYN_SENT)
egress-ca 57555  max   11u  IPv4  0x5413c7234505ae5      0t0  TCP 100.115.3.197:50273->162.159.136.232:443 (SYN_SENT)
COMMAND     PID USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
egress-ca 57555  max    4u  IPv4 0x568dd54b47f5d1a7      0t0  TCP 100.115.3.197:50274->140.82.121.4:443 (ESTABLISHED)
egress-ca 57555  max    5u  IPv4 0xf53251110b2fbe42      0t0  TCP 100.115.3.197:50281->162.159.136.232:443 (SYN_SENT)
egress-ca 57555  max    9u  IPv4 0xba9e9706c48f8da9      0t0  TCP 100.115.3.197:50268->104.16.249.249:443 (ESTABLISHED)
egress-ca 57555  max   10u  IPv4 0x878d83b689572164      0t0  TCP 100.115.3.197:50280->149.154.167.99:443 (ESTABLISHED)
COMMAND     PID USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
egress-ca 57555  max    4u  IPv4 0xc350bb4633b89906      0t0  TCP 100.115.3.197:50285->140.82.121.4:443 (ESTABLISHED)
egress-ca 57555  max    5u  IPv4 0xf53251110b2fbe42      0t0  TCP 100.115.3.197:50281->162.159.136.232:443 (ESTABLISHED)
egress-ca 57555  max    9u  IPv4 0xba9e9706c48f8da9      0t0  TCP 100.115.3.197:50268->104.16.249.249:443 (ESTABLISHED)
egress-ca 57555  max   10u  IPv4  0xd5c6f655247c16d      0t0  TCP 100.115.3.197:50282->149.154.167.99:443 (ESTABLISHED)
egress-ca 57555  max   11u  IPv4 0x6a8bd67221440cbb      0t0  UDP 100.115.3.197:62248->162.159.136.232:33440
egress-ca 57555  max   12u  IPv4 0x31c0c96bd7a43f22      0t0 ICMP *:*
egress-ca 57555  max   13u  IPv4 0x74d9386895651313      0t0  UDP 100.115.3.197:58964->162.159.136.232:33441
egress-ca 57555  max   14u  IPv4 0xf46a4e30b5ce381a      0t0  UDP 100.115.3.197:59204->162.159.136.232:33442
COMMAND     PID USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
egress-ca 57555  max    4u  IPv4 0xc350bb4633b89906      0t0  TCP 100.115.3.197:50285->140.82.121.4:443 (ESTABLISHED)
egress-ca 57555  max    5u  IPv4 0xf53251110b2fbe42      0t0  TCP 100.115.3.197:50281->162.159.136.232:443 (ESTABLISHED)
egress-ca 57555  max    9u  IPv4 0xba9e9706c48f8da9      0t0  TCP 100.115.3.197:50268->104.16.249.249:443 (ESTABLISHED)
egress-ca 57555  max   10u  IPv4  0xd5c6f655247c16d      0t0  TCP 100.115.3.197:50282->149.154.167.99:443 (ESTABLISHED)
```

## Notes

- macOS delegates system name resolution to mDNSResponder, so no resolver socket
  belongs to the probe process; resolver rows remain declared for platforms
  where the ladder queries them directly.
- Trace probes use the standard traceroute UDP port range on the selected target
  address.
- No telemetry, upload or enrichment endpoint appears.
