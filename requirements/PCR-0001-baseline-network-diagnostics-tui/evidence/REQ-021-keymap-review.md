# REQ-021 — Key map review

Frozen requirement: `03-technical.yaml`, REQ-021 "Keyboard navigation and help
overlay". Manual-review evidence: TUI walkthrough notes and key map review
sign-off.

## Reviewed artifacts

- `docs/key-map.md` — every key and page.
- `internal/tui/keys.go` — bindings, footer hints, help content.
- `evidence/REQ-021-keys-tests.txt` — 20 passing tests.
- `evidence/REQ-021-walkthrough.txt` — deterministic renders of every page.

## Checklist

| Criterion (frozen)                                    | State | Where                                                                    |
| :---------------------------------------------------- | :---- | :----------------------------------------------------------------------- |
| A user can review the whole baseline without a mouse. | pass  | every page has keys; no mouse handler exists                             |
| The footer shows contextual key hints.                | pass  | `TestFooterHintsFollowContext`, `TestFooterShowsCancelHintsWhileRunning` |
| Every key is documented in the help overlay.          | pass  | `TestHelpOverlayDocumentsEveryKey`                                       |
| `Esc` only navigates back.                            | pass  | `TestEscOnlyNavigatesBack`                                               |
| Help is reachable from the dashboard.                 | pass  | `TestHelpReturnsToPreviousPage`, transcript                              |
| `Ctrl-C` confirms before quitting.                    | pass  | `TestCtrlCRequiresConfirmation`                                          |

## Review notes

- Navigation: arrows and `j`/`k`, `g`/`G`, `h`/`l` and `[`/`]`, `n`/`p` for
  problem checks, `PgUp`/`PgDn` on scrollable pages.
- Actions: `r` re-run, `e`/`E` export, `x`/`X` cancel, `?` help, `q` quit.
- Pages: dashboard, detail, help, destinations, context, export prompt and the
  quit confirmation.
- The help overlay scrolls, so every key stays documented at 60x20.
- Letter keys are layout-dependent by design: terminals deliver translated
  characters, so no scan-code binding exists in v1.

## Sign-off

Reviewer: gothchibjo, 2026-11-12. The key map matches the implemented bindings
and the frozen REQ-021 text.
