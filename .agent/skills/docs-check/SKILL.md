---
name: docs-check
description: Validates a ThunderID documentation MDX file against all content standards — frontmatter completeness, heading hierarchy, code block language tags, ProductName usage, relative links, Stepper config, and sidebar registration. Also suggests (never fails on) a better sidebar placement when a page is registered but sits in a category with no directory, topic, or doc-type match. Use when creating or reviewing any .mdx file, before merging a docs PR, or when asked to check if a page meets standards.
allowed-tools: Read Bash
---

# ThunderID Docs Standards Checker

Validate a single `.mdx` file against all documented standards. Report every violation clearly.

## Usage

Invoked as `/docs-check [file-path]`

If no path is given, ask the user which file to check. Resolve the absolute path before reading.

If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop and say this skill only checks published documentation content (`docs/content/**` or SDK docs) — a skill file is agent instructions, not a documentation page, and these rules do not apply to it.

---

## Checks to Run

Read the full file content once, then evaluate each rule.

---

### 1. Frontmatter

Parse the YAML block between the opening and closing `---` delimiters.

**`title`**
- Must be present and non-empty.
- ❌ Missing or empty → FAIL

**`description`**
- Must be present and non-empty → FAIL if missing
- Under 70 characters → WARN (likely too thin; Google may ignore it and generate its own snippet)
- Over 200 characters → WARN (likely to get truncated in search results)
- Must NOT start with: "This page", "This guide", "This document", "This section" → FAIL
- Should end with a period → WARN if missing

There is no hard length range. Meta description length is not a Docusaurus requirement or a Google ranking factor — it's an approximate SEO heuristic about search-result snippet truncation, which Google renders by pixel width, not character count. Only the two extremes (too thin to be useful, or long enough to reliably truncate) are worth flagging, and only as warnings. What actually matters for SEO is content quality — see `/docs-seo`'s Check 2.

**`toc_progress: quickstart`**
- REQUIRED if the file body contains `<Stepper` → FAIL if missing
- Must NOT be present if the file does not contain `<Stepper` → FAIL if present without Stepper

---

### 2. Heading Hierarchy

Extract all headings (`#`, `##`, `###`, etc.) in document order. Track the heading level sequence.

A violation occurs when the level jumps by more than 1 step downward:
- H1 → H3 is a skip (H2 missing) → FAIL
- H2 → H4 is a skip (H3 missing) → FAIL
- H1 → H2 → H4 is a skip → FAIL
- H2 → H3 → H2 → H3 is fine (going back up is always allowed)

Report the line numbers of the offending headings.

---

### 3. Code Blocks

Scan for every fenced code block (lines starting with ` ``` ` or `~~~`).

Every opening fence must have a language tag immediately after it (no space before the tag):

```
❌  ```
✅  ```bash
✅  ```ts
```

Common valid language tags: `bash`, `ts`, `tsx`, `js`, `jsx`, `json`, `yaml`, `kotlin`, `swift`, `dart`, `vue`, `http`, `text`, `md`, `html`, `css`, `sql`, `go`, `python`, `java`, `xml`, `toml`

Report the line number of each unlabeled block → WARN. New blocks must always have a language tag; changing all pre-existing unlabeled blocks is out of scope for a single PR.

---

### 4. ProductName in Prose

In prose (outside code blocks), the product name must use `<ProductName />`, not the hardcoded string "ThunderID".

**Exceptions — these are allowed:**
- Inside fenced code blocks
- Package names: `@thunderid/react`, `@thunderid/nextjs`, etc.
- Import paths: `from '@thunderid/...'`
- URLs: `thunderid.dev`, `localhost:8090`

Report each violation with line number → FAIL.

---

### 5. Internal Links

No internal link may use an absolute path starting with `/docs/`.

Scan for:
- Markdown links: `[text](/docs/...)`
- HTML href: `href="/docs/..."`

All internal links must be relative (`../`, `./`, or bare filename).

Report each violation with line number → FAIL.

---

### 6. Stepper Configuration

If the file contains `<Stepper`, check that `stepNode` and `as` attribute values match.

- `stepNode="h2" as="h2"` → OK
- `stepNode="h3" as="h3"` → OK
- `stepNode="h2" as="h3"` → FAIL (mismatch)
- `stepNode="h3" as="h4"` → FAIL (mismatch; a mismatch also risks a visual heading-level skip in the rendered document)
- `<Stepper>` with no attributes → WARN (attributes should be explicit)

---

### 7. Image Alt Text

Scan the file content read in Step 1 for image syntax. For each `![` occurrence, check whether the alt text field is empty:
- `![](...)` — empty alt text → WARN
- `![alt text](...)` — non-empty alt text → OK

All images in documentation must have descriptive alt text. Only intentionally decorative images may use empty alt.

Report each empty alt text instance with line number → WARN.

---

### 8. Line Dividers

`---` is valid only as a frontmatter delimiter (the opening and closing fences of the YAML block). A `---` appearing anywhere in the document body is a horizontal rule used as a visual divider — this is not acceptable in ThunderID documentation. Use section headings to separate content instead.

Scan for any line that is exactly `---` after the frontmatter closing delimiter, excluding lines inside fenced code blocks (which may legitimately show YAML or frontmatter examples).

Report each occurrence with line number → FAIL.

---

### 9. Sidebar Registration and Placement

Every documentation page must appear in `docs/sidebars.ts` or in the relevant SDK sidebar at `docs/content/sdks/<sdk>/sidebar.ts`.

The doc ID for a file is its path relative to `docs/content/`, with the `.mdx` extension removed.

Example: `docs/content/guides/guides/flows/build-a-flow.mdx` → doc ID `guides/guides/flows/build-a-flow`

Search for the doc ID:

```bash
grep -rn "id: '<doc-id>'" docs/sidebars.ts docs/content/sdks/*/sidebar.ts
```

- Match found → OK
- No match, but the ID appears in `.orphan-allowlist` → WARN (pre-existing gap, not a merge blocker)
- No match at all → FAIL

**Placement sanity (judgment-based, WARN only, never a hard gate):** contributors are free to hand-edit `sidebars.ts` directly instead of going through `/docs-new-page` — this check exists so a poor placement still gets caught rather than silently shipping. Once a match is found, evaluate it against the same criteria `/docs-new-page` uses to place a *new* page:

- **Directory match**: does the page sit in a category whose other items share its parent directory?
- **Topic match**: does the category's theme genuinely match the page's subject, not just a coincidental shared keyword?
- **Doc-type match**: is a concept page filed among concepts, a task guide among guides, and so on?

If the placement clearly violates these (e.g., a Kubernetes deployment guide filed under "Identity Providers"), flag it: name the mismatch and propose where it should move, in the same section → category → position format `/docs-new-page` presents for approval. Do not fail the check over this — flag it as a suggestion and move on. A page that's registered but sitting in a slightly awkward spot is a much smaller problem than one that isn't registered at all.

---

## Output Format

Print results as a checklist:

```
Checking: docs/content/guides/getting-started/connect-your-application/react.mdx

FRONTMATTER
  ✅  title: present
  ✅  description: 143 chars — complete sentence
  ✅  toc_progress: quickstart present (Stepper found)

HEADINGS
  ✅  hierarchy: sequential

CODE BLOCKS
  ⚠️   line 43: unlabeled code block
  ⚠️   line 91: unlabeled code block

PRODUCT NAME
  ✅  no hardcoded "ThunderID" in prose

INTERNAL LINKS
  ❌  line 67: absolute link /docs/next/guides/getting-started/build-a-flow

STEPPER
  ✅  stepNode="h2" as="h2" — match

IMAGE ALT TEXT
  ✅  all images have alt text

LINE DIVIDERS
  ✅  no --- in body

SIDEBAR REGISTRATION
  ✅  guides/guides/flows/build-a-flow — registered in sidebars.ts
  ✅  placement: Guides → Flows — directory, topic, and doc-type all match

─────────────────────────────────────
1 failure · 2 warnings
```

If placement looks off:
```
SIDEBAR REGISTRATION
  ✅  guides/deployment-patterns/kubernetes — registered in sidebars.ts
  ⚠️  placement: filed under Guides → Identity Providers, but this is a Kubernetes
      deployment guide with no directory or topic overlap with that category.
      Consider moving it to Deployment Patterns → Deployment Paths instead.
```

If all checks pass:
```
─────────────────────────────────────
✅  All checks passed
```
