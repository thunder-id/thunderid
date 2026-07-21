---
name: docs-check
description: Validates a ThunderID doc MDX file against content standards (frontmatter, headings, code block tags, nested list indentation, ordered vs. unordered lists, ProductName usage, links, Stepper config, sidebar registration). Use before merging any .mdx, or whenever asked to check, lint, or verify a doc page meets standards.
allowed-tools: Read Bash
---

# ThunderID Docs Standards Checker

Validate a single `.mdx` file against all documented standards. Report every violation clearly.

## Usage

Invoked as `/docs-check [file-path]`. If no path is given, ask which file to check. Resolve the absolute path before reading.

If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop: it's agent instructions, not a documentation page (`docs/content/**` or SDK docs), and these rules don't apply.

---

## Checks to Run

Read the full file content once, then evaluate each rule.

---

### 1. Frontmatter

Parse the YAML block between the `---` delimiters.

**`title`**: must be present and non-empty. Missing/empty → FAIL.

**`description`**:
- Must be present and non-empty → FAIL if missing.
- Under 70 chars → WARN (likely too thin; Google may ignore it and generate its own snippet). Over 200 → WARN (likely truncated in results).
- Must not start with "This page/guide/document/section" → FAIL. Should end with a period → WARN if missing.

No hard length range: meta description length isn't a Docusaurus requirement or ranking factor, just an approximate heuristic about snippet truncation (Google renders by pixel width, not char count). Only the two extremes are worth flagging, as warnings. Content quality is what matters for SEO — see `/docs-seo` Check 2.

**`docType`**: must be present and one of `quickstart`, `guide`, `concept`, `reference`, `use-case`, `community` → FAIL if missing or any other value. Lets future work query which doc types exist for a given topic and spot coverage gaps.

---

### 2. Heading Hierarchy

Extract all headings in document order; track the level sequence. A jump of more than 1 level downward is a violation (H1→H3, H2→H4, H1→H2→H4) → FAIL. Going back up (H2→H3→H2→H3) is always fine. Report offending line numbers.

---

### 3. Code Blocks

Every fenced code block's opening fence needs a language tag immediately after it, no space:
```
❌  ```
✅  ```bash
```

Common tags: `bash`, `ts`, `tsx`, `js`, `jsx`, `json`, `yaml`, `kotlin`, `swift`, `dart`, `vue`, `http`, `text`, `md`, `html`, `css`, `sql`, `go`, `python`, `java`, `xml`, `toml`.

Report each unlabeled block's line number → WARN. New blocks always need a tag; retrofitting all pre-existing unlabeled blocks is out of scope for a single PR.

---

### 4. Nested List Indentation

A sub-list (a list nested inside another list item, e.g. sub-steps under one bullet) must indent its marker by exactly 4 spaces from its parent bullet's marker column. Docusaurus's MDX/remark parser doesn't reliably nest a sub-list at 2-space or inconsistent indentation — it either collapses into a flat continuation paragraph (the sub-items silently disappear as a list) or breaks out into a separate top-level list.

❌
```
- Parent item
  - Sub item (2-space indent — fails to nest, renders as plain text or breaks the list)
```
✅
```
- Parent item
    - Sub item (4-space indent — nests correctly)
```

Scan for a bulleted or numbered sub-item indented 1-3 spaces directly under a parent list item → FAIL. Report each occurrence's line number; this is easy to miss by eye but breaks rendering silently, so treat it as a hard check, not a style nit.

---

### 5. Ordered vs. Unordered Lists

A sequence of actions the reader performs in order, especially Console or UI steps, must be a numbered list, not bullets. Bullets are only for unordered sets with no required order: options, characteristics, prerequisites, component descriptions.

❌
```
- Sign in to the Console.
- Navigate to Applications, and click Add Application.
- Under Technology, select React.
```
✅
```
1. Sign in to the Console.
2. Navigate to Applications, and click Add Application.
3. Under Technology, select React.
```

This applies at every nesting level: a sub-list of sub-steps under one numbered step is itself numbered (restarting at 1), not bulleted.

Flag any bulleted list whose items are actions performed in sequence (imperative verbs, dependent order, "first do X then Y") → FAIL. Report line numbers.

---

### 6. ProductName in Prose

In prose (outside code blocks), use `<ProductName />`, not the hardcoded string "ThunderID".

**Exceptions**: fenced code blocks; package names (`@thunderid/react`, etc.); import paths (`from '@thunderid/...'`); URLs (`thunderid.dev`, `localhost:8090`).

Report each violation with line number → FAIL.

---

### 7. Internal Links

No internal link may use an absolute path starting with `/docs/` — check Markdown links (`[text](/docs/...)`) and HTML `href="/docs/..."`. All internal links must be relative. Report each violation with line number → FAIL.

**Do not hand-verify relative link depth by counting directories.** Docusaurus resolves a relative link against the *rendered page's own URL*, which has a trailing slash, so the page's own name becomes an extra pseudo-directory segment. A link written by naive "same folder" counting is routinely off by exactly one level, except for files literally named `index.mdx` (whose URL has no extra segment, so naive counting is correct there). This is subtle enough that manually re-deriving the right number of `../` isn't reliable even when you understand the rule.

The only trustworthy check is running `make build_docs` (Docusaurus build with `onBrokenLinks: 'throw'`) and confirming zero broken links — don't mark a relative-link fix done without that.

---

### 8. Stepper Configuration

If the file contains `<Stepper`, check `stepNode` and `as` match: `stepNode="h2" as="h2"` OK, `stepNode="h2" as="h3"` FAIL (mismatch; also risks a visual heading skip). `<Stepper>` with no attributes → WARN (should be explicit).

---

### 9. Image Alt Text

For each `![` occurrence, empty alt (`![](...)`) → WARN; non-empty → OK. All images need descriptive alt text; only intentionally decorative images may use empty alt. Report each instance with line number.

---

### 10. Line Dividers

`---` is valid only as the frontmatter delimiter. Any `---` in the body (a horizontal-rule divider) is not acceptable — use section headings instead. Scan for exact `---` lines after the frontmatter close, excluding fenced code blocks (which may legitimately show YAML/frontmatter examples). Report each occurrence with line number → FAIL.

---

### 11. Sidebar Registration and Placement

Every page must appear in `docs/sidebars.ts` or the relevant `docs/content/sdks/<sdk>/sidebar.ts`. The doc ID is the file's path relative to `docs/content/`, minus `.mdx` (e.g. `docs/content/guides/guides/flows/build-a-flow.mdx` → `guides/guides/flows/build-a-flow`).

```bash
grep -rn "id: '<doc-id>'" docs/sidebars.ts docs/content/sdks/*/sidebar.ts
```

Match found → OK. No match but ID is in `.orphan-allowlist` → WARN (pre-existing gap, not a blocker). No match at all → FAIL.

**Placement sanity** (judgment-based, WARN only, never a hard gate): contributors can hand-edit `sidebars.ts` directly instead of using `/docs-new-page`, so this catches a poor placement before it ships silently. Once matched, judge it against the same criteria `/docs-new-page` uses for new pages:
- **Directory match**: does its category share a parent directory with its siblings?
- **Topic match**: does the category's theme genuinely match the page's subject, not just a coincidental keyword?
- **Doc-type match**: concepts among concepts, guides among guides, etc.

If it clearly violates these (e.g., a Kubernetes guide filed under "Identity Providers"), flag the mismatch and propose where it should move, in the same section → category → position format `/docs-new-page` uses for approval — as a suggestion, never a failure. A registered page in an awkward spot is a much smaller problem than an unregistered one.

---

## Output Format

Print results as a checklist:

```
Checking: docs/content/guides/getting-started/connect-your-application/react.mdx

FRONTMATTER
  ✅  title: present
  ✅  description: 143 chars — complete sentence
  ✅  docType: quickstart
  ✅  toc_progress: quickstart present (Stepper found)

HEADINGS
  ✅  hierarchy: sequential

CODE BLOCKS
  ⚠️   line 43: unlabeled code block
  ⚠️   line 91: unlabeled code block

NESTED LISTS
  ✅  all sub-lists indented 4 spaces

LIST TYPE
  ✅  step sequences are numbered, bullets used only for unordered sets

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
