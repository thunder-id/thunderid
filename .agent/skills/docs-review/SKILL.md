---
name: docs-review
description: Runs docs-check, docs-review-style, and docs-review-tech on one file; combines results into a pass/fail report with a confidence rating. Use before merging any doc PR, or whenever asked for a full or complete review of a documentation page.
allowed-tools: Read Bash WebFetch WebSearch
---

# ThunderID Docs — Full Review

Orchestrates a complete review of one file. Runs all three review skills in sequence and combines their output into one structured report.

## Usage

Invoked as `/docs-review [file-path]`. If no path is given, ask which file. If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop: it's agent instructions, not a documentation page.

Covers structural standards, writing quality (AI-pattern detection, voice consistency), and technical accuracy. For new pages or significant rewrites, also run `/docs-seo`; its findings are advisory and listed separately.

---

## Review Sequence

Run each skill's checks in full, in order, before compiling the combined report — don't skip to the output format early.

1. **`/docs-check`** — frontmatter, heading hierarchy, code block tags, ProductName usage, internal links, Stepper config, alt text, sidebar registration/placement.
2. **`/docs-review-style`** — AI vocabulary, sentence cadence, promotional tone, generic writing, rhetorical scaffolding, voice vs. siblings, per-rule style (contractions, terminology, formatting, doc-type patterns), step count/decomposition, step ordering.
3. **`/docs-review-tech`** — protocol claims, code examples, API/SDK accuracy, config claims, security claims, diagram-to-text consistency.

Don't skip any — each catches different failure modes.

---

## Combined Output

```
Reviewing: [file path]
Doc type: [detected type]

─────────────────────────────────────
DOCS-CHECK
[Paste the docs-check output. Omit passing checks — show only failures and warnings.]

DOCS-REVIEW-STYLE
[Paste the docs-review-style output. Omit passing checks.]

DOCS-REVIEW-TECH
[Paste the docs-review-tech output. Omit passing checks — keep the category coverage summary.]

─────────────────────────────────────
OVERALL RESULT: PASS / FAIL
Confidence: High / Medium / Low

Hard gate failures: [count] ([which skill(s)])
Failures: [count]
Warnings: [count]
```

If all three pass cleanly, show only the passing summary:
```
DOCS-CHECK
  ✅  All checks passed

DOCS-REVIEW-STYLE
  ✅  Result: PASS — 0 failures · 0 warnings

DOCS-REVIEW-TECH
  ✅  Result: PASS — C:0 · H:0 · M:0 · L:0

─────────────────────────────────────
OVERALL RESULT: PASS
Confidence: High
Hard gate failures: 0
```

---

## Confidence Rating

**High** — all three pass with at most 2 low-severity warnings; ready for merge review.

**Medium** — one or more skills produced non-gate failures or warnings; address before merging.

**Low** — any hard gate failure from any skill; must be revised before merge.

---

## Hard Gate Summary

Overall review FAILS if any of these are true:

From `docs-check`: any ❌ FAIL (title, description length/prefix, heading hierarchy, toc_progress/Stepper consistency, ProductName in prose, absolute links, Stepper config, line dividers).

From `docs-review-style` (its Hard Gate Rules): any em/en dash present; 5+ AI vocabulary violations; any rhetorical scaffolding phrase; 2+ symmetric contrast constructions. (Step Count/Decomposition and Step Locality are also checked there but never gate — they resolve by asking the user, not a mechanical fail.)

From `docs-review-tech`: any CRITICAL issue; 3+ HIGH issues; any unverifiable code example; any security claim unverifiable against two authoritative sources; a cross-page contradiction.

---

## Scope Boundary

| Check | This skill | Underlying skill |
|---|---|---|
| Structural standards | delegates | `/docs-check` |
| Writing quality, voice, AI-pattern detection | delegates | `/docs-review-style` |
| Technical accuracy | delegates | `/docs-review-tech` |
| Page scaffold creation | — | `/docs-new-page` |
