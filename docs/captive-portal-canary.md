---
type: reference
author: gothchibjo
date: 2026-11-12
---

# Captive portal canary <!-- omit in toc -->

- [Endpoint](#endpoint)
- [Expected response](#expected-response)
- [Classification](#classification)
- [Guards](#guards)

## Endpoint

The check implements [REQ-011] and requests
`http://captive.apple.com/hotspot-detect.html` exactly once with `GET`.
Redirects are captured with `http.ErrUseLastResponse` and never followed, so a
portal login page is never loaded.

## Expected response

A healthy path returns status `200 OK` with a body containing both markers:

| Marker                   | Meaning                     |
| :----------------------- | :-------------------------- |
| `<TITLE>Success</TITLE>` | expected Apple canary title |
| `<BODY>Success</BODY>`   | expected Apple canary body  |

Comparison lowercases the body and collapses whitespace before matching. The
check records the observed status, the fingerprint verdict and a bounded body
snippet as evidence.

## Classification

| Observation                    | Status | Diagnostic                                   |
| :----------------------------- | :----- | :------------------------------------------- |
| `200 OK` and both markers      | `pass` | none                                         |
| `3xx` with a `Location` header | `warn` | captive portal redirect with the target      |
| `200 OK` without the markers   | `warn` | body does not match the expected fingerprint |
| any other status               | `warn` | the observed status                          |
| transport or read failure      | `fail` | the underlying error                         |

## Guards

- The check makes one HTTP request and never follows a redirect.
- No authentication, form submission or network configuration change occurs.
- The request carries no credentials and no portal data is stored.

<!-- refs -->

[REQ-011]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
