# ThunderID New Doc Page Scaffold

Create a new `.mdx` file, register it in the sidebar in the best available position, and verify the result. The file should be ready to fill in with content and CI should pass — no follow-up steps required.

## Usage

Read when the user asks to create, add, or write documentation for a new guide, quickstart, concept, reference, or use-case page — including requests like "document this feature" or "write docs for X" when no existing page covers the topic.

## Phase 1: Collect Information

Collect everything upfront; if anything is missing, ask for all of it in one message, not one question at a time.

1. **Page type** — one of `quickstart`, `guide`, `concept`, `reference`, `use-case`. Written to frontmatter as `docType`.
2. **Title** — for `title:` frontmatter and the H1.
3. **Description** — the meta description, roughly 70-200 chars (neither is a hard limit; too thin and Google may ignore it, too long and it truncates), must not start with "This page/guide/document". Draft one and confirm if the user hasn't provided it.
4. **File path** — relative to `docs/content/`. Infer from context if obvious ("a new React quickstart" → `getting-started/connect-your-application/react.mdx`).

Don't ask for `sidebar_position` — derive it in Phase 2.

### Check for an Existing Page First

Before creating anything (after collecting the title, before Phase 2), search for a page that might already cover this topic.

Search by title/topic (2-3 significant title words, skip generic ones like "guide"/"overview"/"configure"):
```bash
grep -rli "{KEY_TERM}" docs/content --include="*.mdx" 2>/dev/null
grep -rn "^title:\|^# " docs/content --include="*.mdx" | grep -i "{KEY_TERM}"
```
Sidebar labels can differ from page titles, so also check them:
```bash
grep -n "label:" docs/sidebars.ts docs/content/sdks/*/sidebar.ts | grep -i "{KEY_TERM}"
```

For each result, read enough (title, description, headings) to judge genuine topic overlap, not just a shared keyword — both pages mentioning "authentication" doesn't make them duplicates.

If a genuine match is found, stop and ask:
> I found an existing page that may already cover this: **{EXISTING_TITLE}** (`{EXISTING_PATH}`). Do you want to edit that page instead of creating a new one, or is this a genuinely different page?

Include the title, path, and a one-line summary. Wait for the answer: **edit existing** → stop this skill, edit that page instead following `edit.md`; **different page** → continue to Phase 2.

If no genuine match, say so briefly and continue.

## Phase 2: Create the MDX File

**Derive `sidebar_position`:**
```bash
grep -r "sidebar_position:" docs/content/<target-directory>/ 2>/dev/null | grep -oP '(?<=sidebar_position: )\d+' | sort -n | tail -1
```
Use the highest existing value + 1, or `1` if no siblings have one.

**Create the file** using the template for the page type (see Templates below).

**Do not invent content.** Only fill in what the user explicitly gave or confirmed. Don't author prerequisites, section titles, technical steps, or links based on assumptions about what the topic probably needs — that's authoring, not scaffolding.
- Leave bracketed placeholders (`{FIRST_SECTION}`, `{content}`, etc.) exactly as-is unless the user specified them.
- Keep the template's default `Prerequisites` line as-is; don't add bullets unless stated.
- Don't add extra links/further reading/`Next Steps` entries beyond the template default unless provided.
- A well-founded guess (e.g. from a sibling page's structure) goes in your reply as a suggestion to confirm, never written directly into the file.

**Diagrams:** use a fenced ` ```mermaid ` block; never raw SVG/ASCII art; no per-diagram color overrides (brand colors apply site-wide via `docusaurus.config.ts` → `themeConfig.mermaid`). Full rule: `style.md`.

## Phase 3: Place in Sidebar

Mandatory — a page without a sidebar entry fails CI.

### Placement Principles

Grounded in the Diátaxis framework and comparable identity platforms (Auth0, Clerk). Apply alongside the Step 4 match signals — these resolve what the signals alone don't.

- **Reference lives next to what it supports**, not in an isolated reference silo — a `reference` page belongs in the same category as the `guide`/`quickstart` a reader consults it alongside (why `sdks/<sdk>/apis/` sits next to `sdks/<sdk>/guides/`).
- **Concept pages are ordered after the practical content they support**, not before, unless truly foundational (the reader can't attempt the task at all without it).
- **Use-case pages group by reader scenario, not technical topic** (e.g. "B2C," "AI Agents," "B2B," cutting across doc types).
- **Match placement to what the reader is doing at that moment**, not just shared keywords — a reader debugging a token issue and one learning what tokens are may belong in different sections on the same subject.

### Step 1: Read the sidebar
Read `docs/sidebars.ts` in full; for `docs/content/sdks/<sdk>/` pages, also read `docs/content/sdks/<sdk>/sidebar.ts`.

### Step 2: Determine the target sidebar file
| File path prefix | Edit this file |
|---|---|
| `docs/content/sdks/<sdk>/` | `docs/content/sdks/<sdk>/sidebar.ts` |
| `docs/content/community/` | `docs/sidebars.ts` → `communitySidebar` |
| Everything else | `docs/sidebars.ts` → `docsSidebar` |

### Step 3: Determine the target section
For `docsSidebar`:
| File path starts with | Section |
|---|---|
| `getting-started/` | **Get Started** |
| `working-with-ai/` | **Working with AI** |
| `use-cases/` | **Use Cases** |
| `guides/` | **Guides** |
| `key-concepts/` | **Key Concepts** |
| `deployment/` | **Deployment** |

No prefix match → pick the section whose existing pages are most topically related.

### Step 4: Choose the category
Find the category whose existing items are most topically related, in priority order:
1. **Directory match** — the new file's parent directory matches a category's existing items' parent directory.
2. **Topic match** — title/content overlaps a category's theme.
3. **Doc type match** — concepts in Key Concepts, task guides in Guides.

**If no category matches:** also check whether the target directory itself is new:
```bash
ls docs/content/{TARGET_DIRECTORY} 2>/dev/null
```
If it doesn't exist or has no other `.mdx` files, note this creates a new folder too — carry that into Step 7's approval message (a flag to surface, not a reason to force a worse-fitting category). Create a new category at the section level, matching the pattern of existing categories:
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
Insert in topic order relative to existing categories (the conceptual sequence a reader encounters them), not alphabetically.

### Step 5: Determine position within the category
Insert: after any prerequisite topics the new page depends on; before any follow-on topics that naturally lead to it; a `concept` page defaults to after the `guide`/`quickstart` items it explains (per Placement Principles), unless it's a genuine prerequisite; otherwise at the end. Don't insert alphabetically unless the whole category already is.

### Step 6: Determine the sidebar label
The label appears in the left nav. Shorter than the full title is fine. Match sibling grammar: task guides use an imperative verb phrase ("Add Google"), concepts a noun phrase ("Authorization"), reference a descriptive noun ("Client Authentication Methods"). Don't include the product name (already in context). Max ~40 characters.

### Step 7: Get user approval before editing
Before touching `docs/sidebars.ts` (or the SDK's `sidebar.ts`), present and wait for approval:
- Section → category → position (what it goes after/before, or "new category")
- The exact label (Step 6)
- The exact snippet to insert (Step 8)
- If Step 4 flagged a new folder: one line stating that and why no existing folder fit. Omit entirely when placing into an existing folder.

If the user wants a different section/category/position/label, revise and re-present. Don't proceed to Step 8 without explicit approval.

### Step 8: Apply the sidebar edit
Doc ID = file path relative to `docs/content/`, minus `.mdx` (e.g. `docs/content/guides/flows/build-a-flow.mdx` → `guides/flows/build-a-flow`).

```ts
{type: 'doc', id: '{DOC_ID}', label: '{LABEL}'},
```
Or wrapped in a new category (Step 4):
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

**Make the edit:** find a unique anchor string immediately before the insertion point (typically the closing `},`/`}` of the preceding sibling item, with enough surrounding lines to be unambiguous). Use Edit with that anchor as `old_string`, replaced with the same anchor plus the new entry.

Example, inserting after the last item in a category:
```
old_string:
            {type: 'doc', id: 'guides/flows/advanced-configurations', label: 'Advanced Configurations'},
          ],

new_string:
            {type: 'doc', id: 'guides/flows/advanced-configurations', label: 'Advanced Configurations'},
            {type: 'doc', id: 'guides/flows/build-a-flow', label: 'Build a Flow'},
          ],
```

If appending to the section itself, use the closing of the last category as the anchor.

## Phase 4: Verify

```bash
./scripts/docs-lint.sh docs/content/{FILE_PATH}
```
Check "Sidebar orphans" — no new `❌` for the new page. If it's still flagged, the sidebar edit didn't take; re-read `docs/sidebars.ts` and fix it.

## Phase 5: Report to the User

Tell the user: file created (`docs/content/{FILE_PATH}`); sidebar entry added (section → category → label, plus the exact TypeScript snippet); lint result (PASS or warnings); remaining `{PLACEHOLDER}`s that need content; that a full quality check (`review.md`) is available once it's filled in.

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

**Stepper import path:** `src/` lives at `docs/src/`. Count directory levels from the target file up to `docs/content/`, prefix with that many `../`, append `src/components/Stepper`. To avoid counting by hand:
```bash
grep -r "import Stepper" docs/content/<same-directory>/ | head -1
```
Copy the import path from a neighboring quickstart. First step is always "Run ThunderID", last is always "What's Next".

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
