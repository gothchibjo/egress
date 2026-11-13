---
type: reference
author: gothchibjo
date: 2026-11-13
---

# Preset target catalog <!-- omit in toc -->

- [Catalog](#catalog)
- [Defaults](#defaults)
- [Custom targets](#custom-targets)
- [Versioning](#versioning)

## Catalog

The catalog ships inside the binary (`internal/domain/target.go`) and loads with
no configuration file and no network access.

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

## Defaults

Every preset enables the same probe protocols by default: ICMP echo, TCP
connect, TLS ClientHello and HTTP GET. The default ports are the TCP ports of
the ladder; DNS resolution always runs first and ICMP does not use a port.

## Custom targets

The picker accepts `host[:port][@protocol]`:

- host — hostname, IPv4 literal, IPv6 literal, or bracketed `[IPv6]` when a port
  follows;
- port — optional, 1-65535;
- protocol — optional override: `tcp`, `tls`, `http` or `icmp`.

Examples: `example.com`, `example.com:8443`, `[2001:db8::1]:443@tls`,
`1.2.3.4:53@tcp`.

Full URLs with a scheme or path are rejected. Hostnames are resolved before the
run and the addresses are shown in the picker; an unresolvable name stays
selectable and becomes a diagnostic result during the run. Custom targets are
de-duplicated by host, port and protocol.

## Versioning

The catalog version is the constant `domain.CatalogVersion`, reported in every
session and included in the session JSON export. The catalog is static in v1:
there is no update mechanism and no remote fetch.

<!-- refs -->
