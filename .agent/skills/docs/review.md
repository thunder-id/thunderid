# ThunderID Docs — Full Review

Orchestrates a complete review of one file, or of every changed doc file (diff mode). Runs all three review references in sequence and combines their output into one structured report.

## Usage

Read when the user asks for a full or complete review of a documentation page, or before merging any doc PR.

- **A file path is given** → whole-file mode (below).
- **No file path, and the user says "review my changes," "review the diff," or just invokes review with nothing else** → diff mode (below).
- **No file path and no diff context either** (nothing changed, or ambiguous) → ask which file.

Covers structural standards, writing quality (AI-pattern detection, voice consistency), and technical accuracy. For new pages or significant rewrites, also run `seo.md`; its findings are advisory and listed separately.

If the target is an OpenAPI spec (`api/*.yaml`) or an SDK reference page (`docs/content/sdks/*/apis/**`), also run `api.md` — unlike `seo.md` this is **not advisory**; its hard gates count toward the overall result the same way `tech.md`'s do.

---

## Diff Mode

Reviews every changed `.mdx`/`.md` file instead of one named file — including uncommitted changes, since a lot of review requests happen before anything's committed.

### Step 1: Determine the base ref

Default to `main`. If `main` doesn't exist locally, or the user names a different target branch (e.g. reviewing against a release branch), use that instead.

```bash
BASE_REF=main
MERGE_BASE=$(git merge-base "$BASE_REF" HEAD)
```

### Step 2: Find every changed file

One diff against the merge-base — with no second ref, `git diff` compares against the working tree, which already includes both committed changes since the merge-base *and* uncommitted (staged + unstaged) changes in a single pass:

```bash
git diff --name-only --diff-filter=ACMR "$MERGE_BASE" -- '*.md' '*.mdx' ':!.agent/skills/**' ':!.claude/skills/**'
```

That misses brand-new files git doesn't know about yet, so also list untracked ones:

```bash
git status --porcelain -- '*.md' '*.mdx' | awk '$1=="??"{print $2}'
```

Combine and de-duplicate both lists into the file set for this review. If it's empty, say so and stop — there's nothing to review.

### Step 3: Run the deterministic pass

`scripts/docs-lint.sh` already implements exactly this scoping (new files checked in full, existing files scoped to lines actually changed since the merge-base, uncommitted included) for everything in it that's mechanically checkable — most of `check.md`'s rules plus the Vale-covered parts of `style.md`:

```bash
DOCS_LINT_BASE_REF="$MERGE_BASE" ./scripts/docs-lint.sh <changed-files...>
```

Fold this output into the CHECK/STYLE sections of the combined report below instead of re-deriving those same findings by eye.

### Step 4: Run the judgment-only checks, scoped to the diff

`docs-lint.sh` can't automate everything `check.md`, `style.md`, and `tech.md` cover (Stepper config, sidebar placement judgment, voice calibration, rhetorical scaffolding, AI structural patterns, all of technical accuracy). For each changed file:

- **New file** (not present at the merge-base): apply every check in full, same as whole-file mode.
- **Existing file**: read `git diff -U3 "$MERGE_BASE" -- <file>` for readable context, and apply the checks only to the changed lines/sections. Don't re-flag pre-existing issues elsewhere in the file — that's not what this diff touched.

### Step 5: Report per file, then an overall summary

Use the same Combined Output format below, once per changed file, with the header replaced:

```
Reviewing diff: [file path] (vs [base ref], includes uncommitted changes)
```

Then a final rollup: total files reviewed, total failures/warnings across all of them, and the overall PASS/FAIL.

---

## Review Sequence

Run each reference's checks in full, in order, before compiling the combined report — don't skip to the output format early.

1. **`check.md`** — frontmatter, heading hierarchy, code block tags, ProductName usage, internal links, Stepper config, alt text, sidebar registration/placement.
2. **`style.md`** — AI vocabulary, sentence cadence, promotional tone, generic writing, rhetorical scaffolding, voice vs. siblings, per-rule style (contractions, terminology, formatting, doc-type patterns), step count/decomposition, step ordering.
3. **`tech.md`** — protocol claims, code examples, API/SDK accuracy, config claims, security claims, diagram-to-text consistency.
4. **`api.md`** (only when the target is `api/*.yaml` or `docs/content/sdks/*/apis/**`) — endpoint/schema accuracy against the backend, or SDK signature/behavior accuracy against build artifacts, plus API-reference-specific structure and coverage.

Don't skip any that apply — each catches different failure modes.

---

## Combined Output

```
Reviewing: [file path]
Doc type: [detected type]

─────────────────────────────────────
CHECK
[Paste the check.md output. Omit passing checks — show only failures and warnings.]

STYLE
[Paste the style.md output. Omit passing checks.]

TECH
[Paste the tech.md output. Omit passing checks — keep the category coverage summary.]

API
[Only present when api.md ran. Paste its output. Omit passing checks.]

─────────────────────────────────────
OVERALL RESULT: PASS / FAIL
Confidence: High / Medium / Low

Hard gate failures: [count] ([which reference(s)])
Failures: [count]
Warnings: [count]
```

If all three pass cleanly, show only the passing summary:
```
CHECK
  ✅  All checks passed

STYLE
  ✅  Result: PASS — 0 failures · 0 warnings

TECH
  ✅  Result: PASS — C:0 · H:0 · M:0 · L:0

─────────────────────────────────────
OVERALL RESULT: PASS
Confidence: High
Hard gate failures: 0
```

---

## Confidence Rating

**High** — every reference that ran passes with at most 2 low-severity warnings; ready for merge review.

**Medium** — one or more references produced non-gate failures or warnings; address before merging.

**Low** — any hard gate failure from any reference; must be revised before merge.

---

## Hard Gate Summary

Overall review FAILS if any of these are true:

From `check.md`: any ❌ FAIL (title, description length/prefix, heading hierarchy, toc_progress/Stepper consistency, ProductName in prose, absolute links, Stepper config, line dividers).

From `style.md` (its Hard Gate Rules): any em/en dash present; 5+ AI vocabulary violations; any rhetorical scaffolding phrase; 2+ symmetric contrast constructions. (Step Count/Decomposition and Step Locality are also checked there but never gate — they resolve by asking the user, not a mechanical fail.)

From `tech.md`: any CRITICAL issue; 3+ HIGH issues; any unverifiable code example; any security claim unverifiable against two authoritative sources; a cross-page contradiction.

From `api.md` (when it ran): any documented REST endpoint with no matching backend route; any schema field contradicting the backend struct; any missing summary/tag description; invalid YAML or a dangling `$ref`; any SDK page whose signature block and table disagree; any claim contradicting available build artifacts; any named error not in the SDK's error registry; any UNVERIFIED claim with no stated reason.

---

## Scope Boundary

| Check | This reference | Underlying reference |
|---|---|---|
| Structural standards | delegates | `check.md` |
| Writing quality, voice, AI-pattern detection | delegates | `style.md` |
| Technical accuracy | delegates | `tech.md` |
| API doc consistency and accuracy (OpenAPI specs, SDK reference pages) | delegates (when the path matches) | `api.md` |
| Page scaffold creation | — | `new-page.md` |
