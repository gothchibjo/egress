# REQ-025 — Terminal matrix

Frozen requirement: `03-technical.yaml`, REQ-025 "Dashboard rendering and
adaptive layout". Manual-review evidence: TUI walkthrough notes and this
terminal matrix.

## Sizes

| Size   | Layout    | Verified behaviour                                   | Test                                                                    |
| :----- | :-------- | :--------------------------------------------------- | :---------------------------------------------------------------------- |
| 120x40 | normal    | list and evidence side by side                       | `TestNormalLayoutShowsBothPanes`                                        |
| 100x30 | normal    | columns, scope band, footer hints                    | `TestLayoutThresholds`, transcript                                      |
| 80x24  | normal    | minimum normal layout                                | `TestLayoutThresholds`                                                  |
| 79x24  | compact   | panes stacked, selection kept                        | `TestCompactLayoutStacksPanes`                                          |
| 60x20  | compact   | minimum supported size, help and destinations scroll | `TestHelpFitsMinimumTerminal`, `TestContextPageScrollsOnShortTerminals` |
| 59x20  | too small | message with the minimum size, no partial render     | `TestTooSmallMessage`                                                   |
| 39x10  | stopped   | nothing rendered                                     | `TestRenderingStopsBelowMinimum`                                        |

## Colour and encoding

| Situation       | Verified behaviour                                     | Test                               |
| :-------------- | :----------------------------------------------------- | :--------------------------------- |
| `NO_COLOR`      | no ANSI escapes, status still readable through symbols | `TestNOColorIsHonored`             |
| ANSI profile    | styled output, identical visible layout                | `TestColorProfiles`                |
| ANSI256 profile | styled output, identical visible layout                | `TestColorProfiles`                |
| ASCII fallback  | `-`, `\|` and `[ok]`/`[!!]`/`[xx]`/`[--]` markers      | `TestASCIIFallback`                |
| monochrome      | status never depends on colour alone                   | `TestStatusIsReadableWithoutColor` |

## Behaviour across sizes

| Behaviour                                                   | Verification                                                                                            |
| :---------------------------------------------------------- | :------------------------------------------------------------------------------------------------------ |
| resize keeps the selected check                             | `TestResizePreservesSelection`, `TestUpdateHandlesWindowSize`                                           |
| terminal height is always filled                            | `TestViewKeepsTerminalHeightWithBand`, `TestExportKeepsTerminalHeight`, `TestDetailKeepsTerminalHeight` |
| long evidence wraps and scrolls                             | `TestLongEvidenceWrapsInsidePane`, `TestDetailScrollsLongEvidence`                                      |
| scope band wraps or truncates                               | `TestScopeBandWrapsLongSummary`, `TestScopeBandTruncatesWithEllipsis`                                   |
| destination table drops the purpose column below 96 columns | `TestDestinationsPageDropsPurposeWhenItDoesNotFit`                                                      |

## Evidence artifacts

- `evidence/REQ-025-layout-tests.txt` — the layout, colour and resize tests.
- `evidence/REQ-021-walkthrough.txt` — deterministic renders at 100x30, 60x20
  and 50x15, including every page.

## Sign-off

Reviewer: gothchibjo, 2026-11-12. The terminal matrix and the walkthrough
transcript (`REQ-021-walkthrough.txt`) match the rendered dashboard.
