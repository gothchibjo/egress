# REQ-008 — Traceability matrix

| Layer          | Reference                                                        |
| :------------- | :--------------------------------------------------------------- |
| Business rules | BR-02 — Baseline coverage; BR-15 — Proxy and VPN context         |
| Requirement    | REQ-008 — Proxy environment detection (must, xs)                 |
| Implementation | `internal/platform/proxy.go` — `ProxyConfig`, `DetectProxyEnv()` |
| Test           | `internal/platform/proxy_test.go` — five cases                   |
| Test report    | `evidence/REQ-008-unit-tests.txt`                                |
| Guard          | no proxy bypass or direct-versus-proxied comparison — pass       |

## Requirement to implementation

| REQ-008 element                          | Implementation                                                                                      |
| :--------------------------------------- | :-------------------------------------------------------------------------------------------------- |
| Read proxy environment variables         | `DetectProxyEnv()` over `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, `NO_PROXY` and lowercase variants |
| Report each setting with its source      | `ProxySetting.Source` keeps the variable actually read                                              |
| State that none is set                   | `ProxyConfig.Empty()` plus a non-nil empty `Settings` slice                                         |
| Do not bypass the detected proxy         | detection only; no transport, no bypass path                                                        |
| Detection present in TUI and JSON output | pending integration                                                                                 |

## Test coverage

| Case                                    | Test                                        |
| :-------------------------------------- | :------------------------------------------ |
| Empty environment                       | `TestDetectProxyEnvEmpty`                   |
| Uppercase variables                     | `TestDetectProxyEnvUppercase`               |
| Lowercase variables report their source | `TestDetectProxyEnvLowercaseSources`        |
| Both case variants present              | `TestDetectProxyEnvReportsBothCaseVariants` |
| Credentials redacted                    | `TestDetectProxyEnvRedactsCredentials`      |
| Process environment is read             | `TestDetectProxyEnvReadsProcessEnvironment` |

## Open evidence

| Missing artifact                       | Owner requirement |
| :------------------------------------- | :---------------- |
| TUI and JSON exposure of proxy context | REQ-023, REQ-022  |
| Explicit "none set" presentation       | REQ-023           |
