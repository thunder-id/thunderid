---
name: docs-new-page
description: Scaffolds a new ThunderID documentation MDX file, places it in the correct position in the sidebar, and verifies no orphan is created. One command produces a compliant file and a registered sidebar entry. Use when creating any new guide, quickstart, concept, reference, or use-case page.
allowed-tools: Read Write Bash Edit
---

# ThunderID New Doc Page Scaffold

Create a new `.mdx` file, register it in the sidebar in the best available position, and verify the result. The file should be ready to fill in with content and CI should pass — no follow-up steps required.

## Usage

Invoked as `/docs-new-page`

## Phase 1: Collect Information

Collect everything upfront. If the user has not provided all of the following, ask for the missing items in a single message — do not ask one question at a time.

1. **Page type** — one of: `quickstart`, `guide`, `concept`, `reference`, `use-case`. Written to frontmatter as `docType` (see templates below).
2. **Title** — the page title (used for `title:` frontmatter and the H1)
3. **Description** — the meta description. Should be roughly 70–200 characters (too thin and Google may ignore it; too long and it gets truncated in search results — neither is a hard limit) and must not start with "This page/guide/document". If the user has not written one, draft one and confirm before proceeding.
4. **File path** — where to create the file, relative to `docs/content/`. Infer from context if obvious (e.g. "a new React quickstart" → `guides/getting-started/connect-your-application/react.mdx`).

Do not ask for `sidebar_position` — derive it in Phase 2.

### Check for an Existing Page First

Before creating anything, search for a page that might already cover this topic. Do this after collecting the title, before Phase 2.

**Search by title and topic:**
```bash
grep -rli "{KEY_TERM}" docs/content --include="*.mdx" 2>/dev/null
```
Run this for the 2–3 most significant words in the title (skip generic words like "guide", "overview", "configure"). Also check frontmatter titles and H1 headings directly:
```bash
grep -rn "^title:\|^# " docs/content --include="*.mdx" | grep -i "{KEY_TERM}"
```

Sidebar labels can differ from page titles, so also check them:
```bash
grep -n "label:" docs/sidebars.ts docs/content/sdks/*/sidebar.ts | grep -i "{KEY_TERM}"
```

**Evaluate matches:** For each result, read enough of the file (title, description, headings) to judge whether it covers the same topic as the requested page, not just a shared keyword. Both pages mentioning "authentication" does not make them duplicates; the core subject and doc type must genuinely overlap.

**If a genuine match is found**, stop and ask the user:

> I found an existing page that may already cover this: **{EXISTING_TITLE}** (`{EXISTING_PATH}`). Do you want to edit that page instead of creating a new one, or is this a genuinely different page?

Include the existing page's title, path, and a one-line summary of what it covers. Wait for the user's answer before proceeding:

- **Edit the existing page** — stop this skill. Suggest `/docs-edit {EXISTING_PATH}` or point the user to the file directly.
- **Different page, create new anyway** — continue to Phase 2.

**If no genuine match is found**, state that briefly and continue to Phase 2 without asking.

## Phase 2: Create the MDX File

**Derive `sidebar_position`:**

Run:
```bash
grep -r "sidebar_position:" docs/content/<target-directory>/ 2>/dev/null | grep -oP '(?<=sidebar_position: )\d+' | sort -n | tail -1
```
Use the highest existing value plus 1. If no siblings have a `sidebar_position`, use `1`.

**Create the file** using the template for the detected page type (see Templates section below).

**Do not invent content.** Only fill in what the user explicitly gave you or explicitly confirmed: title, description, page type, file path, and anything else stated directly in the request. Do not author prerequisites, section titles, technical steps, or links based on assumptions about what the topic probably needs — that is authoring, not scaffolding.

- Leave bracketed template placeholders (`{FIRST_SECTION}`, `{SECOND_SECTION}`, `{content}`, etc.) exactly as they appear in the template unless the user told you what those sections should be.
- Keep the template's default `Prerequisites` line as-is. Do not add further prerequisite bullets unless the user stated them.
- Do not add extra links, further reading, or additional `Next Steps` entries beyond the template default unless the user provided them.
- If you have a well-founded guess (e.g., inferred from a sibling page's structure), state it as a suggestion in your reply and ask for confirmation — do not write it into the file directly.

**Diagrams:** If the page will include a flow, architecture, or sequence diagram, use a fenced ` ```mermaid ` code block. Do not hand-build diagrams with raw SVG or ASCII art, and do not add per-diagram color overrides (`%%{init: {'theme': ...}}%%`, inline `style`/`classDef` colors) — the brand color scheme is applied site-wide via `docusaurus.config.ts` → `themeConfig.mermaid`. See `/docs-review-style` for the full rule.

## Phase 3: Place in Sidebar

This phase is mandatory. A page without a sidebar entry fails CI.

### Placement Principles

These are grounded in the Diátaxis documentation framework and in how comparable identity platforms (Auth0, Clerk) structure their docs. Apply them alongside the match signals in Step 4 — they resolve cases the signals alone don't, not replace them.

- **Reference lives next to what it supports, not in an isolated reference silo.** A `reference` page belongs in the same category/folder as the `guide` or `quickstart` a reader would consult it alongside, not moved to a generic reference-only area. This is why `sdks/<sdk>/apis/` sits next to `sdks/<sdk>/guides/` for the same SDK — keep that pattern.
- **Concept pages are ordered after the practical content they support, not before**, unless the concept is truly foundational (the reader cannot attempt the task at all without it). A reader usually reaches for a `concept` page to understand something *after* hitting a real task, not as a mandatory prerequisite gate.
- **Use-case pages group by reader scenario, not by technical topic.** A `use-case` page belongs with other pages describing the same kind of reader or problem (e.g., "B2C," "AI Agents," "B2B"), cutting across doc types, rather than filed under the technical feature it happens to use.
- **Match placement to what the reader is doing at that moment, not just shared keywords.** Two pages both mentioning "authentication" are not necessarily neighbors — a reader debugging a token issue and a reader learning what tokens are are in different moments, and may belong in different sections even on the same subject.

### Step 1: Read the sidebar

Read `docs/sidebars.ts` in full. For a page under `docs/content/sdks/<sdk>/`, also read `docs/content/sdks/<sdk>/sidebar.ts`.

### Step 2: Determine the target sidebar file

| File path prefix | Edit this file |
|---|---|
| `docs/content/sdks/<sdk>/` | `docs/content/sdks/<sdk>/sidebar.ts` |
| `docs/content/community/` | `docs/sidebars.ts` → `communitySidebar` section |
| Everything else | `docs/sidebars.ts` → `docsSidebar` section |

### Step 3: Determine the target section

For `docsSidebar`, map the file path to the correct top-level section:

| File path starts with | Section |
|---|---|
| `guides/getting-started/` | **Get Started** |
| `guides/working-with-ai/` | **Working with AI** |
| `use-cases/` | **Use Cases** |
| `guides/guides/` | **Guides** |
| `guides/key-concepts/` | **Key Concepts** |
| `guides/deployment-patterns/` | **Deployment Patterns** |

If the path does not match any prefix, choose the section whose existing pages are most topically related to the new page.

### Step 4: Choose the category

Within the target section, find the category whose existing items are most topically related to the new page.

**Match signals — in priority order:**

1. **Directory match**: the new file's parent directory matches a category's existing items' parent directory. Example: `guides/guides/flows/new-page.mdx` belongs in the Flows category because all other Flows items share that prefix.
2. **Topic match**: the new page's title and content overlap with a category's theme (e.g., a page about OAuth grant types belongs in Guides → Protocols & Standards → OAuth & OIDC → Grant Types).
3. **Doc type match**: concept pages belong in Key Concepts; task guides belong in Guides.

**If no category matches:**

This is also the moment to check whether the target directory is new. Run:
```bash
ls docs/content/{TARGET_DIRECTORY} 2>/dev/null
```
If this directory does not exist or has no other `.mdx` files in it, note that placing the page here also creates a new folder, not just a new category — carry this into Step 7's approval message. Do not treat this as reason to avoid a new folder; it is only a flag to surface, not a reason to force the page into a worse-fitting existing category.

Create a new category at the section level. Use the pattern of existing categories in that section:
```ts
{
  type: 'category',
  label: '{CATEGORY_LABEL}',
  collapsed: true,
  collapsible: true,
  items: [
    {
      type: 'doc',
      id: '{DOC_ID}',
      label: '{LABEL}',
    },
  ],
},
```
Insert the new category in topic order relative to existing categories — not alphabetically, but in the conceptual sequence a reader would encounter them.

### Step 5: Determine position within the category

Read the items already in the target category. Insert the new item:

- **After any prerequisite topics**: if the new page depends on understanding a concept already in the category, place it after that concept.
- **Before any follow-on topics**: if existing pages naturally lead to the new page's topic, place the new page before them.
- **If the new page is a `concept`**: default to placing it after the `guide`/`quickstart` items in the category that it explains, per the Placement Principles above — not before them — unless it's a genuine prerequisite the reader needs before attempting the task at all.
- **At the end** if no ordering signal exists.

Do not insert alphabetically unless the entire category is already alphabetically ordered.

### Step 6: Determine the sidebar label

The label is what appears in the left nav. Rules:

- Shorter than the full page title is fine.
- Match the grammatical pattern of sibling labels:
  - Task guides: imperative verb phrase ("Add Google", "Configure the Redirect URI", "Build a Flow")
  - Concepts: noun phrase ("Authorization", "Passkeys", "Token Formats")
  - Reference: descriptive noun ("Client Authentication Methods", "Token Lifetime Configuration Options")
- Do not include the product name in the label — it is already in context.
- Maximum ~40 characters.

### Step 7: Get user approval before editing

Before touching `docs/sidebars.ts` (or the SDK's `sidebar.ts`), present the placement to the user and wait for approval:

- Section → category → position (what it goes after/before, or "new category")
- The exact label chosen (Step 6)
- The exact snippet that will be inserted (Step 8)
- If Step 4 flagged a new folder: one line stating that, plus why no existing folder fit (e.g., "No existing directory covers this topic, so this also creates `docs/content/{dir}/`."). Omit this line entirely when placing into an existing folder — do not mention folders on every run, only when one is actually being created.

If the user requests a different section, category, position, or label, revise and re-present before editing. Do not proceed to Step 8 without explicit approval.

### Step 8: Apply the sidebar edit

The doc ID is the file path relative to `docs/content/` with `.mdx` removed.
Example: `docs/content/guides/guides/flows/build-a-flow.mdx` → `guides/guides/flows/build-a-flow`

Construct the entry to insert:
```ts
{type: 'doc', id: '{DOC_ID}', label: '{LABEL}'},
```

Or for a newly created category wrapping the entry (from Step 4):
```ts
{
  type: 'category',
  label: '{CATEGORY_LABEL}',
  collapsed: true,
  collapsible: true,
  items: [
    {type: 'doc', id: '{DOC_ID}', label: '{LABEL}'},
  ],
},
```

**Make the edit:**

Find a unique anchor string immediately before the insertion point — typically the closing `},` or `}` of the preceding sibling item, with enough surrounding lines to be unambiguous. Use Edit with that anchor as `old_string` and replace it with the same anchor plus the new entry.

Example — inserting after the last item in a category:
```
old_string:
            {type: 'doc', id: 'guides/guides/flows/advanced-configurations', label: 'Advanced Configurations'},
          ],

new_string:
            {type: 'doc', id: 'guides/guides/flows/advanced-configurations', label: 'Advanced Configurations'},
            {type: 'doc', id: 'guides/guides/flows/build-a-flow', label: 'Build a Flow'},
          ],
```

If the insertion point is at the end of a section's items array (appending to the section itself), use the closing of the last category as the anchor.

## Phase 4: Verify

Run:
```bash
./scripts/docs-lint.sh docs/content/{FILE_PATH}
```

Check the "Sidebar orphans" section of the output. It must show no new `❌` errors for the new page. If the orphan check still flags the page, the sidebar edit did not take — re-read `docs/sidebars.ts` and fix the entry.


## Phase 5: Report to the User

Tell the user:

- File created: `docs/content/{FILE_PATH}`
- Sidebar entry added: location in sidebar (section → category → label) and the exact TypeScript snippet added
- Lint result: PASS or any warnings to address
- Placeholders in the file that need content (list each `{PLACEHOLDER}` remaining)
- Command to run a full quality check once they have filled in the content: `/docs-review docs/content/{FILE_PATH}`


## Templates

---

### `quickstart`

A step-by-step guide for connecting a technology to ThunderID. Always uses a `<Stepper>`.

```mdx
---
title: {TITLE}
docType: quickstart
toc_progress: quickstart
sidebar_position: {POSITION}
description: {DESCRIPTION}
---

import Stepper from '{STEPPER_IMPORT_PATH}';

# {TITLE}

{ONE_LINE_INTRO}

<Stepper stepNode="h2" as="h2">

## Run ThunderID

{content}

## Create an Application

{content}

## What's Next

{content}

</Stepper>
```

**Stepper import path:** The `src/` directory lives at `docs/src/`. Count directory levels from the target file up to `docs/content/`, then prefix with that many `../` and append `src/components/Stepper`. To avoid counting by hand:
```bash
grep -r "import Stepper" docs/content/<same-directory>/ | head -1
```
Copy the import path from a neighboring quickstart. The first step is always "Run ThunderID", the last always "What's Next".

---

### `guide`

```mdx
---
title: {TITLE}
docType: guide
sidebar_position: {POSITION}
description: {DESCRIPTION}
---

# {TITLE}

{INTRO}

## Prerequisites

- ThunderID running locally. See [Get ThunderID](../getting-started/get-thunderid).

## {FIRST_SECTION}

{content}

## {SECOND_SECTION}

{content}

## Next Steps

- [{NEXT_STEP_TITLE}]({RELATIVE_LINK})
```

---

### `concept`

```mdx
---
title: {TITLE}
docType: concept
sidebar_position: {POSITION}
description: {DESCRIPTION}
---

# {TITLE}

{INTRO}

## How It Works

{content}

## When to Use This

{content}

## Related

- [{RELATED_PAGE}]({RELATIVE_LINK})
```

---

### `reference`

```mdx
---
title: {TITLE}
docType: reference
sidebar_position: {POSITION}
description: {DESCRIPTION}
---

# {TITLE}

{ONE_LINE_SUMMARY}

## {FIRST_SECTION}

| Field | Type | Description |
|-------|------|-------------|
| | | |

## {SECOND_SECTION}

{content}
```

---

### `use-case`

```mdx
---
title: {TITLE}
docType: use-case
sidebar_position: {POSITION}
description: {DESCRIPTION}
---

# {TITLE}

{INTRO}

## When to Choose This Pattern

This pattern is a good fit when you need to:

- {CRITERION_1}
- {CRITERION_2}

## How It Works

{content}

## Try It Out

{LINK_TO_TRY_IT_OUT}
```
