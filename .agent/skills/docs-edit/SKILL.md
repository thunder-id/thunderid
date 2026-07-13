---
name: docs-edit
description: Writes new content into a ThunderID documentation MDX file — a scaffolded placeholder from `/docs-new-page`, a gap in a page a human has partly written, or a new section requested for an already-complete page. Can also take a user-supplied draft, design doc, or notes as an input source, cross-checked against the codebase rather than trusted blindly. Gated on verifying every technical claim against the codebase, existing docs, a standard spec, or the supplied draft before drafting it; asks the user for specifics rather than fabricating anything unverifiable. Also checks step-by-step content before finalizing it — for oversized step sequences or step ordering that bounces between UI screens with no dependency forcing it, proposes a restructuring and asks the user which way to go rather than blocking. Use whenever new prose needs to be written into a doc, regardless of how much of the page already exists. For reviewing existing prose quality, use `/docs-review-style`.
allowed-tools: Read Edit Bash WebFetch
---

# ThunderID Docs Writing

Writes new content into an `.mdx` file. The target can be a literal `{placeholder}` left by `/docs-new-page`, an unwritten or thin section in a page a human has otherwise finished, or a brand new section added to a page that is already complete. **Never fabricate a technical claim** — an empty section, a human's rough draft, or a request to "add a section on X" is not permission to guess at how ThunderID works. Do not touch structure, frontmatter completeness, or link format — those are for `/docs-check`. For reviewing existing prose quality, use `/docs-review-style` instead.

## Usage

Invoked as `/docs-edit [file-path]`

If no path is given, ask which file to work on. State what needs writing: a literal placeholder, a specific gap the user points out, or a new section they've asked for. If none of these is obvious from the request, ask which part of the file needs content before proceeding.

If the target is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop and say this skill only writes published documentation content — editing a skill's own instructions is a different, meta-level task this skill is not scoped for.

The user can also hand over a draft, a design doc, meeting notes, or a PRD alongside the target file, either pasted directly or as a path to read. Treat this as a fourth verification source in Step 2, not as a replacement for Steps 1-4 — a supplied draft still gets classified section by section, and any section the draft does not cover still goes through the normal verify-or-ask process.

---

## Step 1: Identify what each section actually needs

Before writing anything, state in one line what factual or technical claim the section requires — regardless of whether it's a literal `{content}` placeholder, an empty or thin section in an otherwise-finished page, or a new section a human asked you to add. A vague target like "## Configure the Exporter" (empty, half-written, or requested fresh) needs a concrete claim: "the exact config keys and values for exporting to X."

---

## Step 2: Classify each claim as verifiable or not

For every claim a section needs, check it against, in this order:

1. **A draft, design doc, or notes the user explicitly supplied for this task** — treat a claim stated there as user-confirmed, the same as an answered Step 3 question. If the claim is also easy to check against the codebase, still check it: a stale or wrong draft is exactly the kind of thing this skill exists to catch. If the codebase contradicts the draft, do not silently pick a side — tell the user both versions and ask which is correct before drafting that claim. If the claim genuinely cannot be checked independently (a forward-looking statement, a product decision, something not yet implemented), the draft alone is sufficient confirmation.
2. **The ThunderID codebase** — grep for the relevant executor, config schema, or API handler. This is the strongest independent source; cite the file path you verified it against.
3. **An existing, correct doc page** — if another page already documents this claim accurately, reuse it (link or summarize) instead of re-deriving it.
4. **A stable, well-known external standard** — an RFC, W3C spec, or similarly established protocol document. Only use this for claims about the *standard itself* (e.g., "PKCE requires a code_verifier and code_challenge"). Claims about how ThunderID specifically implements a standard still need source #2.

If a claim is confirmed through any of these, draft the content and note what you verified it against — match the evidence standard `/docs-review-tech` holds existing content to. When the source is a user-supplied draft, say so plainly (e.g., "per the design doc you provided") rather than presenting it as independently verified.

---

## Step 3: If a claim isn't verifiable, stop and ask — specifically

Do not write speculative content, and do not ask a vague "what should I write here?" Name the exact fact you're missing and ask for it directly:

> I can't verify **{the specific claim}** from the codebase, existing docs, a standard spec, or anything in the draft you provided. To write this section accurately, I need:
> - {specific piece of information needed}
> - {specific piece of information needed, if more than one}
>
> Once you provide this, I'll draft the section. I won't write about how this works without it.

Batch every missing-information question for the file into one message, the same way `/docs-new-page` collects information upfront — do not interrupt the user once per unverifiable claim.

---

## Step 4: Draft only what's confirmed

Write the sections you have verified information for. For anything still blocked by an unanswered question: leave a literal `{placeholder}` as-is if that's what it was, or simply do not add the requested section/sentence yet if it was a human-written page or a fresh request — say what's still missing rather than half-writing content around the gap.

### Deciding whether a diagram belongs here

Do not draw a diagram just because a section could theoretically have one.

- **If you're the one who thinks a diagram would help, and the user didn't ask for one**: stop and ask first. State what kind of diagram (sequence, architecture, flow) and why it would help, then wait for confirmation before drawing it.
- **If the user explicitly asks you to draw a diagram**:
  - If it's genuinely necessary — a multi-actor exchange, several interacting components, a sequence of steps across systems — draw it.
  - If it doesn't look necessary — a single linear list, a purely conceptual point that reads fine as prose — say so plainly, give your reasoning, and ask "Are you sure you want a diagram here?" before drawing one. If the user confirms, draw it; do not keep arguing the point.

When a diagram is going ahead, do not hand-build it with raw SVG or ASCII art — use a fenced ` ```mermaid ` code block, and do not add per-diagram color overrides (the brand color scheme is applied site-wide via `docusaurus.config.ts` → `themeConfig.mermaid`). See `/docs-review-style` for the full diagram rule.

Do not use emojis anywhere in what you write — no decorative icons in front of terms, in table cells, or in headings. See `/docs-review-style` for the full rule.

### Checking step count and screen locality before finalizing

Before writing a final Stepper or numbered step sequence, count the steps and check which UI location each one targets. Neither check blocks you from writing the content — both resolve by asking the user, not by refusing to proceed. If both apply to the same sequence, ask about them together in one message rather than two separate questions.

- **10 or more steps in one sequence**: propose a specific restructuring (split across pages, group into labeled phases, or fold trivial steps into a neighbor) and ask: *"Do you think the flat step-by-step structure works here, or would restructuring it as proposed be clearer?"* If the user prefers the flat structure, write it as-is.
- **Back-and-forth between UI locations with no dependency forcing it**: if two steps targeting the same screen end up separated by a step targeting a different one, and nothing depends on that ordering, propose the regrouped sequence and ask: *"Do you think this order is the right way to go, or should the same-screen steps be grouped together?"* Write whichever the user confirms.

Both are drafting-time applications of the same criteria `/docs-review-style` checks (Step Count and Decomposition, Step Locality) — surfacing them while writing means the user makes the call once, up front, instead of getting flagged again on review.

Unlike `/docs-review-style`, this skill can write directly, so when the user confirms a structure, add the matching confirmation marker as part of the same edit — immediately above the Stepper or numbered list:
- Flat structure confirmed: `<!-- docs-review-style: step-count-confirmed steps={N} -->`
- Current screen order confirmed: `<!-- docs-review-style: step-locality-confirmed screens={ordered list, e.g. Applications,Settings} -->`

This means a page drafted here and confirmed once won't get re-asked the same question the next time `/docs-review-style` or `/docs-review` runs on it, as long as the step count or screen order hasn't changed since.
