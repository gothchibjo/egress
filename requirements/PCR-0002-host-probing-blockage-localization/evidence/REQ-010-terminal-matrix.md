# REQ-010 — Run view terminal matrix

Manual-review evidence for the adaptive run view: TUI walkthrough notes and this
terminal matrix.

## Sizes

| Size   | Layout    | Verified behaviour                                      | Test                                      |
| :----- | :-------- | :------------------------------------------------------ | :---------------------------------------- |
| 120x40 | normal    | target tabs, ladder and hops side by side, verdict band | `TestRunViewWideShowsLadderAndHops`       |
| 100x30 | normal    | same split; `3` opens the full verdict                  | `TestRunViewVerdictShowsLabelledEvidence` |
| 80x24  | normal    | minimum normal layout                                   | `TestRunViewKeepsTerminalHeight`          |
| 60x20  | compact   | one pane at a time, `1`/`2`/`3` switch it               | `TestRunViewCompactSwitchesPanes`         |
| 59x20  | too small | resize message, no partial render                       | `TestTooSmallMessage`                     |
| 39x10  | stopped   | nothing rendered                                        | `TestRenderingStopsBelowMinimum`          |

## Colour and encoding

| Situation      | Verified behaviour                                  | Test                         |
| :------------- | :-------------------------------------------------- | :--------------------------- |
| `NO_COLOR`     | status readable through markers and words           | `TestNOColorIsHonored`       |
| ANSI / ANSI256 | both profiles render, layout unchanged              | `TestColorProfiles`          |
| ASCII fallback | `[ok]`/`[!!]`/`[xx]`/`[--]` markers and ASCII rules | `TestASCIIFallback`          |
| monochrome     | target state and step status never rely on colour   | `TestRunViewShowsTargetTabs` |

## Behaviour

| Behaviour                                | Verification                                                 |
| :--------------------------------------- | :----------------------------------------------------------- |
| tabs show running/done/cancelled         | `TestRunViewShowsTargetTabs`, `TestRunViewShowsLiveProgress` |
| target column keeps every target visible | `TestRunViewTargetColumnAtCompact`                           |
| running target shows its current step    | `TestRunViewShowsPlannedAndRunningSteps`                     |
| queued and finished steps are visible    | `TestRunViewShowsPlannedAndRunningSteps`                     |
| target switching keeps the view          | `TestRunViewSwitchesTargets`                                 |
| scrolling follows the selection          | `TestRunViewScrolling`                                       |
| help returns to the run view             | `TestRunViewHelpReturns`                                     |
| `esc` never cancels                      | `TestRunViewEscNeverCancels`                                 |
| `x`/`X` cancel one or all                | `TestRunViewCancelsTargets`                                  |

## Evidence artifacts

- `evidence/REQ-010-run-view-tests.txt` — the run-view tests.
- `evidence/REQ-010-walkthrough.txt` — deterministic renders at 120x40, 60x20,
  100x30 and 50x15.

## Sign-off

Pending review by gothchibjo.
