---
name: docs-edit
description: Writes new content into an existing ThunderID doc MDX file (placeholder, unfinished section, or new section on a complete page). Verifies technical claims against the codebase/docs/specs before writing; never fabricates. Use whenever asked to write, draft, add, or fill in documentation content on a page that already exists. For a brand-new page, use `/docs-new-page` first. For prose quality review, use `/docs-review-style`.
allowed-tools: Read Edit Bash WebFetch
---

# ThunderID Docs Writing

Writes new content into an `.mdx` file. The target can be a literal `{placeholder}` left by `/docs-new-page`, an unwritten/thin section in an otherwise-finished page, or a brand-new section on a complete page. **Never fabricate a technical claim** — an empty section, a rough draft, or "add a section on X" is not permission to guess how ThunderID works. Structure, frontmatter, and link format are `/docs-check`'s job; prose quality review is `/docs-review-style`'s.

## Usage

Invoked as `/docs-edit [file-path]`. If no path is given, ask which file. State what needs writing (placeholder, a gap the user pointed out, or a requested new section); if unclear from the request, ask.

If the target is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop: this skill writes published docs, not skill instructions.

The user may hand over a draft, design doc, notes, or a PRD alongside the target file. Treat it as a fourth verification source in Step 2, not a replacement for Steps 1-4 — it still gets classified section by section, and anything it doesn't cover still goes through verify-or-ask.

---

## Step 1: Identify what each section actually needs

Before writing, state in one line the factual/technical claim the section requires, whatever its origin (placeholder, thin section, or fresh request). A vague target like "## Configure the Exporter" needs a concrete claim: "the exact config keys and values for exporting to X."

---

## Step 2: Classify each claim as verifiable or not

Check each claim against, in order:

1. **A user-supplied draft/design doc/notes** — treat a stated claim as confirmed, like an answered Step 3 question. Still cross-check against the codebase if easy to; a stale or wrong draft is exactly what this skill exists to catch. If the codebase contradicts the draft, don't pick a side silently — show the user both and ask which is correct. A forward-looking or not-yet-implemented claim can rely on the draft alone.
2. **The ThunderID codebase** — grep for the executor, config schema, or API handler. Strongest source; cite the file path.
3. **An existing, correct doc page** — reuse (link or summarize) instead of re-deriving.
4. **A stable external standard** (RFC, W3C spec) — only for claims about the standard itself ("PKCE requires a code_verifier and code_challenge"). Claims about how ThunderID implements a standard still need source #2.

Confirmed → draft the content and note what you verified it against, matching `/docs-review-tech`'s evidence standard. If the source is a user draft, say so plainly ("per the design doc you provided") rather than presenting it as independently verified.

---

## Step 3: If a claim isn't verifiable, stop and ask — specifically

Don't write speculative content or ask a vague "what should I write here?" Name the exact missing fact:

> I can't verify **{the specific claim}** from the codebase, existing docs, a standard spec, or anything in the draft you provided. To write this section accurately, I need:
> - {specific piece of information needed}
> - {specific piece of information needed, if more than one}
>
> Once you provide this, I'll draft the section. I won't write about how this works without it.

Batch every missing-information question for the file into one message (like `/docs-new-page`'s upfront batching) — don't interrupt once per claim.

---

## Step 4: Draft only what's confirmed

Write the sections you have verified information for. For anything still blocked: leave a literal `{placeholder}` as-is, or simply don't add the section/sentence yet on a human-written page — say what's missing rather than half-writing around the gap.

### Write to the Style Rules, Not Just the Facts

Before drafting, read `.claude/skills/docs-review-style/SKILL.md`'s Step 3 (Universal Checks) and, once you know the page's doc type, its Step 4 entry for that type. Write directly to those rules as you draft — active voice, second person, no condescension/hedging, one action per sentence, no AI vocabulary/filler/superficial -ing tails/over-explaining, no em or en dashes (use its comma/colon rules instead), US spelling, no contractions, correct number/UI-element formatting, no informal abbreviations, consistent terminology, inclusive language, and no AI-sounding structural patterns (rhetorical scaffolding, promotional tone, generic writing) — instead of writing loosely and leaving it for `/docs-review-style` to catch afterward.

This doesn't replace the review step — `/docs-review-style` (or `/docs-review`) still runs before merge — it just means that run should find little or nothing to fix.

### Deciding whether a diagram belongs here

Don't draw one just because a section theoretically could have one.
- **You think a diagram would help, user didn't ask**: stop and ask first — state the kind (sequence, architecture, flow) and why, then wait for confirmation.
- **User explicitly asks for one**: draw it if genuinely necessary (multi-actor exchange, several interacting components, a cross-system step sequence). If it doesn't look necessary (a linear list, a purely conceptual point), say so, give your reasoning, and confirm before drawing — don't keep arguing if they confirm.

When going ahead: use a fenced ` ```mermaid ` block, never raw SVG/ASCII art, and no per-diagram color overrides (brand colors apply site-wide via `docusaurus.config.ts` → `themeConfig.mermaid`). Full rule: `/docs-review-style`.

No emojis anywhere you write, including table cells and headings. Full rule: `/docs-review-style`.

### Checking step count and screen locality before finalizing

Before writing a final Stepper or numbered sequence, count steps and check each one's UI location. Neither blocks writing — both resolve by asking the user. If both apply to the same sequence, ask together in one message.

- **10+ steps in one sequence**: propose a restructuring (split across pages, group into labeled phases, fold trivial steps into a neighbor) and ask whether the flat structure or the proposal works better. Write whichever they confirm.
- **Back-and-forth between UI locations with no dependency forcing it**: if two same-screen steps are separated by a different-screen step with nothing depending on the order, propose the regrouped sequence and ask. Write whichever they confirm.

Both are drafting-time applications of what `/docs-review-style` checks later (Step Count and Decomposition, Step Locality) — surfacing them now means the user decides once, not again on review.

Unlike `/docs-review-style`, this skill writes directly, so add the matching confirmation marker as part of the same edit, immediately above the Stepper/list:
- `<!-- docs-review-style: step-count-confirmed steps={N} -->`
- `<!-- docs-review-style: step-locality-confirmed screens={ordered list, e.g. Applications,Settings} -->`

A page confirmed here won't get re-asked by `/docs-review-style` or `/docs-review` later, as long as the step count or screen order hasn't changed.
