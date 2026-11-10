# Project conventions for AI agents <!-- omit in toc -->

- [Project](#project)
- [Repository layout](#repository-layout)
- [Requirements workflow](#requirements-workflow)
- [Architecture: DDD + SDD](#architecture-ddd--sdd)
- [Git and the index](#git-and-the-index)
- [Go conventions](#go-conventions)
- [Document formatting](#document-formatting)
  - [Quick reference](#quick-reference)
  - [Scope and exceptions](#scope-and-exceptions)
  - [File naming](#file-naming)
  - [Markdown basics](#markdown-basics)
  - [Tables](#tables)
  - [Quotes](#quotes)
  - [Reference-style links](#reference-style-links)
  - [Table of contents](#table-of-contents)
  - [YAML front matter](#yaml-front-matter)
  - [Conciseness](#conciseness)
  - [No invented entities](#no-invented-entities)
  - [Emphasis](#emphasis)
- [Pitfalls](#pitfalls)

## Project

`egress` is a Go terminal UI for diagnosing network blockages. It runs a
baseline health check at startup and, from the dashboard, probes preset or
custom hosts to localize the hop where traffic stops and classify the blockage
type.

Development follows Domain-Driven Design (DDD) and Spec-Driven Development
(SDD). The `req-*` skill family from the [skills repository] produces
specifications; implementation starts only from a frozen technical contract.

Repository language is English: code, comments, CLI output, documents and commit
messages. PART A in `01-vision.md` is the only verbatim, source-language
exception.

Do not invent module paths, package layout or build commands before the
technical spec defines them.

## Repository layout

| Path                             | Contents                                                                          |
| :------------------------------- | :-------------------------------------------------------------------------------- |
| `requirements/`                  | Requirements records and the PCR registry                                         |
| `requirements/README.md`         | Registry: one row per PCR with title, status and update date                      |
| `requirements/PCR-NNNN-<slug>/`  | One requirements record                                                           |
| `.../01-vision.md`               | Canonical status, PART A verbatim vision, PART B append-only clarification log    |
| `.../02-business.md`             | Business requirements, assumptions ledger, revisions                              |
| `.../03-technical.yaml`          | Machine-readable technical specification and pipeline gates                       |
| `.../04-plan.md`                 | Development plan, guard logs and verification evidence (created by `req-develop`) |
| `AGENTS.md`                      | This file                                                                         |
| `LICENSE`                        | MIT license                                                                       |
| `.prettierrc`, `.prettierignore` | Prettier configuration for Markdown                                               |
| `.markdownlint-cli2.jsonc`       | markdownlint-cli2 configuration                                                   |

## Requirements workflow

The canonical state of a PCR lives in
`requirements/PCR-NNNN-<slug>/01-vision.md` front matter:
`registered → elicitation → br_review → br_approved → technical → frozen`.

- No development starts before the technical contract is frozen.
- No requirement is implemented before its plan is approved.
- A violated guard blocks the step until it is resolved.
- A requirement is not closed without verification evidence.
- Frozen specification text is never edited. A scope change is a new PCR.
- History is append-only: questions, revisions, guard logs and evidence only
  accumulate.
- Artifacts are written in English. PART A stays verbatim in the source language
  and is never translated, normalized or reflowed.
- The `req-*` skill family (`req-workflow`, `req-register`, `req-elicitation`,
  `req-technical-spec`, `req-develop`, `req-plan`, `req-guards`,
  `req-verification`) is the only specification pipeline.
- Use `/requirements` for shaping and freezing; use `/develop` for execution
  after `frozen`.
- After every state transition, update the registry row in
  `requirements/README.md`.

## Architecture: DDD + SDD

DDD supplies the domain model and boundaries; SDD supplies the contract that
implementation must satisfy. Neither is optional, and neither replaces the
requirements workflow.

- The frozen `03-technical.yaml` is the implementation contract. Code is written
  against its requirements, not against chat history or assumptions.
- Every implementation unit traces to a `REQ-*` id from the frozen spec. A scope
  change is a new PCR, never an edit to frozen text.
- The ubiquitous language comes from the business glossary and the technical
  spec. Use the same terms in code, package names, tests, CLI help and
  documents; do not introduce synonyms.
- Bounded contexts and their boundaries come from the spec. Do not invent a
  package layout, split the domain or name contexts before the spec defines
  them.
- Keep domain logic independent of the terminal UI, OS probing and I/O. UI and
  probe adapters depend on the domain, not the other way around.
- Prefer explicit domain types and typed errors over primitives and string
  matching where the spec defines a concept.
- No code without a frozen spec; no requirement closed without evidence.

## Git and the index

The repository is initialized on `main` with no commits yet. The index and
history belong to the user.

- Never run `git commit`, `git commit --amend`, `git push`, `git rebase -i`, or
  create a pull request without an explicit instruction from the user. "Work is
  finished" or "everything is ready" is not approval to commit.
- Never run `git add`, `git rm --cached`, `git restore --staged`, `git reset` or
  `git restore` without an explicit instruction.
- Never use `--no-verify`, force-push, or amend a commit that hooks rejected.
- Write files; let the user review, stage and commit.

## Go conventions

When the Go module is created:

```bash
gofmt -w <changed .go files>
go vet ./...
go test ./...
go build ./...
```

- Treat `go.mod` as the source of truth for the Go version.
- Wrap errors with context and honor per-probe `context` deadlines.
- Unit tests must not depend on external network access. Network-dependent
  checks are integration tests and are opt-in.
- Do not add a dependency or a code generator without a stated reason.

## Document formatting

These rules apply to every Markdown file written here.

### Quick reference

```bash
# Format only the files actually changed, to keep the diff scoped.
npx prettier --write <changed .md files>

# Lint the whole repository read-only after every .md edit.
npx markdownlint-cli2 "**/*.md" --no-globs

# Optional: detect drift without writing.
npx prettier --check "**/*.md"
```

### Scope and exceptions

Configuration lives in `.prettierrc`, `.prettierignore` and
`.markdownlint-cli2.jsonc` at the repository root.

- Requirement records follow the `req-workflow` schema. The document-title, TOC
  and `type`/`author` front-matter rules do not apply to
  `requirements/PCR-*/02-business.md` and `03-technical.yaml`; their structure
  is fixed by the requirements process.
- Record Markdown under `requirements/PCR-*/` is excluded from markdownlint
  because its heading hierarchy (multiple H1 sections and H3 requirements) is
  fixed by `req-workflow`. Typographic rules still apply where Prettier is used.
- `requirements/PCR-*/01-vision.md` is excluded from Prettier as well. PART A
  must stay byte-for-byte as received; formatting would rewrap it.
- `AGENTS.md` and the root `README.md` are meta files and do not need YAML front
  matter.
- Everything else follows the rules below.

### File naming

- Document files use English kebab-case with no spaces.
- Requirement records keep their fixed names: `PCR-NNNN-<slug>`, `01-vision.md`,
  `02-business.md`, `03-technical.yaml`, `04-plan.md`. Never rename them.
- Never renumber a PCR or move a record between directories without an explicit
  instruction.

### Markdown basics

- Let Prettier handle wrapping and spacing.
- Headings, lists and code blocks have a blank line before and after.
- Code blocks inside lists are indented.
- No trailing whitespace.
- Use one H1 per document, except requirement records where the structure is
  fixed by `req-workflow`.
- Use HTML only for comments, anchors and GitHub alerts that the format
  requires.

### Tables

- Every table has an alignment row with `:---`, `:---:` or `---:`.
- Numeric columns are right-aligned (`---:`); text columns are left-aligned
  (`:---`).
- Values of the same type in one column use the same number of decimal places.
- Use a non-breaking space (U+00A0) between a value and its unit (for example
  `99.90 %`).
- Table cells use spaces around content: `| cell |`.
- Trace tags in requirements records (`[V §1]`, `[CL-1 Q2]`,
  `[PCR-0002 CL-2 Q2]`) are evidence identifiers, not table cells or links. Keep
  them as written.

### Quotes

- Use straight double quotes (`"`) only. Angle quotes (`«»`) are not allowed.
- The exception is PART A in `01-vision.md`, which preserves the source text
  exactly.

### Reference-style links

Use reference-style links for cross-references in body text:

```markdown
... according to [the technical specification] ...
...
<!-- refs -->
[the technical specification]: 03-technical.yaml
```

- In text: `[link text]` with no extra reference key.
- Definitions go at the very end of the document after the `<!-- refs -->` line.
- The key must match the displayed link text.
- Inline links are allowed only in a TOC.
- Trace tags (`[V §1]`, `[CL-1 Q2]`) are not links and are not converted.

### Table of contents

- A document with more than two or three headings needs a TOC after the H1.
- The H1 carries `<!-- omit in toc -->` at the end of the line.
- TOC entries use inline links: `- [Section](#section)`.
- Requirement records are exempt because their structure is fixed by
  `req-workflow`.

### YAML front matter

- General documents carry `type`, `author` and at least one identifying field.
  The author is `gothchibjo` unless the user says otherwise.
- The date field is named `date`. Do not use `created`, `updated` or
  `last-updated`; modification history belongs to git.
- Requirement records keep their own front-matter schema. Do not add `type` or
  `author` to `01-vision.md`, `02-business.md` or `03-technical.yaml`.
- A body H1 that duplicates the YAML `title` is followed by
  `<!-- markdownlint-disable-file MD025 -->`.

### Conciseness

Text written into repository files is dense, factual and unemotional:

- No filler words, duplicated statements, or a summary that restates the same
  fact.
- No empty comments, notes, callouts or footnotes.
- No emotional or promotional wording and no hedging.
- No narration of the work done ("this section was updated"). Git tracks
  history.

### No invented entities

- Never invent a document code, person, date, version, metric, process, PCR
  number, or reference. A missing code is a reason to ask, not to assign the
  next number.
- Never add files, sections, registry rows or glossary terms that were not
  requested.
- If information is missing or ambiguous, ask a clarifying question.
- Mark assumptions separately from facts; never mix them in one sentence.

### Emphasis

- Italic (`*text*`) is for logical emphasis: first use of a defined term,
  syntactic stress, or distinguishing element names.
- Bold (`**text**`) is for structural references: step names in numbered lists,
  defined terms in bullet lists.
- Do not use bold for field labels, section headers or dates.

## Pitfalls

- **Prettier and PART A.** Prettier rewraps long lines, which would break the
  byte-for-byte vision. Vision files are in `.prettierignore`.
- **Markdownlint and PART A.** The long source paragraph would violate MD013.
  Vision files are ignored by markdownlint for the same reason.
- **Trace tags are not links.** `[V §1]` and `[CL-1 Q2]` are evidence
  identifiers; do not turn them into reference-style links.
- **No TOC in requirement records.** Their structure is owned by `req-workflow`;
  adding a title H1 or TOC can break the schema.
- **No front matter on meta files.** `AGENTS.md` and root `README.md` are
  configuration/documentation entry points, not formal documents.
- **Non-breaking space before units.** Use U+00A0 in tables to prevent an
  unwanted line break between a value and its unit.

<!-- refs -->

[skills repository]: https://github.com/gothchibjo/skills
