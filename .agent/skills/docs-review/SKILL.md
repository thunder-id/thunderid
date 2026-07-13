---
name: docs-review
description: Structural, writing quality, and technical accuracy review for ThunderID — applies docs-check, docs-review-style, and docs-review-tech on a single file and produces one combined pass/fail report with a confidence rating. Use before merging any doc PR.
allowed-tools: Read Bash WebFetch WebSearch
---

# ThunderID Docs — Full Review

Orchestrates a complete review of a single ThunderID documentation file. Runs all three review skills in sequence and combines their output into one structured report.

## Usage

Invoked as `/docs-review [file-path]`

If no path is given, ask which file to review.

If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop and say this skill only reviews published documentation content — a skill file is agent instructions, not a documentation page.

This skill covers structural standards, writing quality (including AI-writing-pattern detection and voice consistency), and technical accuracy. For new pages or significant rewrites, also run `/docs-seo` to check discoverability — its findings are advisory and listed separately.

---

## Review Sequence

Execute each skill's checks in full, in the order listed below. Read each skill's instructions and apply every check to the target file before moving to the next skill. Do not skip to the output format — run all checks first, then compile the combined report.

1. **`/docs-check`** — structural standards: frontmatter, heading hierarchy, code block language tags, ProductName usage, internal links, Stepper config, image alt text, sidebar registration, and (as a suggestion, not a gate) sidebar placement fit
2. **`/docs-review-style`** — writing quality and consistency: AI vocabulary, sentence cadence, promotional tone, generic writing, rhetorical scaffolding, voice calibration against sibling pages, per-rule style (contractions, terminology, formatting, doc-type patterns), step count/decomposition, and step ordering across UI screens
3. **`/docs-review-tech`** — technical accuracy: protocol claims, code examples, API/SDK accuracy, configuration claims, security claims, diagram-to-text consistency

Do not skip any skill. Each catches different failure modes.

---

## Combined Output

After all three reviews are complete, produce a single combined report:

```
Reviewing: [file path]
Doc type: [detected type]

─────────────────────────────────────
DOCS-CHECK
[Paste the docs-check output block. Omit passing checks — show only failures and warnings.]

DOCS-REVIEW-STYLE
[Paste the docs-review-style output block. Omit passing checks — show only failures and warnings.]

DOCS-REVIEW-TECH
[Paste the docs-review-tech output block. Omit passing checks — show failures, warnings, and the category coverage summary.]

─────────────────────────────────────
OVERALL RESULT: PASS / FAIL
Confidence: High / Medium / Low

Hard gate failures: [count] ([which skill(s)])
Failures: [count]
Warnings: [count]
```

If all three reviews pass cleanly, show only the passing summary for each:

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

**High** — All three skills pass with at most 2 low-severity warnings. The doc is ready for merge review.

**Medium** — One or more skills produced non-gate-triggering failures or warnings. Address the flagged items before merging.

**Low** — Any hard gate failure from any skill. The doc must be revised before it can merge.

---

## Hard Gate Summary

The overall review FAILS if any of the following are true:

From `docs-check`:
- Any ❌ FAIL (see docs-check for the full list: title, description length/prefix, heading hierarchy, toc_progress/Stepper consistency, ProductName in prose, absolute links, Stepper config, line dividers)

From `docs-review-style` (see its Hard Gate Rules):
- Any em dash or en dash present
- 5 or more AI vocabulary violations remaining
- Any rhetorical scaffolding phrase present
- More than one symmetric contrast construction

(Step Count and Decomposition and Step Locality are also checked by `docs-review-style` but never contribute to this list — they resolve by asking the user whether their structure is intentional, not by a mechanical fail.)

From `docs-review-tech`:
- Any CRITICAL issue
- 3 or more HIGH issues
- Any code example that cannot be verified
- Any security claim that cannot be verified against two authoritative sources
- A cross-page contradiction

---

## Scope Boundary

| Check | This skill | Underlying skill |
|---|---|---|
| Structural standards | delegates | `/docs-check` |
| Writing quality, voice, AI-pattern detection | delegates | `/docs-review-style` |
| Technical accuracy | delegates | `/docs-review-tech` |
| Page scaffold creation | | `/docs-new-page` |
