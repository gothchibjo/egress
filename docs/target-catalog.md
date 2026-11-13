---
type: reference
author: gothchibjo
date: 2026-11-13
---

# Preset target catalog <!-- omit in toc -->

- [Catalog](#catalog)
- [Defaults](#defaults)
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

## Versioning

The catalog version is the constant `domain.CatalogVersion`, reported in every
session and included in the session JSON export. The catalog is static in v1:
there is no update mechanism and no remote fetch.

<!-- refs -->
