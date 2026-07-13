---
name: docs-seo
description: SEO review for ThunderID documentation pages. Checks whether the page title matches developer search intent, whether the meta description is click-worthy, whether H2 headings use searchable phrasing, and whether the page has a clear enough topic focus to rank. Does not check writing quality or technical accuracy — use docs-review-style and docs-review-tech for those.
allowed-tools: Read Bash
---

# ThunderID Docs — SEO Review

You are reviewing a ThunderID documentation page for search discoverability. Your job is to check whether a developer searching for this page's topic would find it, click it, and land in the right place.

This is not keyword stuffing or term density analysis. ThunderID docs use correct technical language by default — the terms are already there. This review checks whether the page is *structured and titled* in a way that matches how developers phrase their searches.

All findings are warnings. There are no hard gates — SEO is advisory. The author decides whether to act.

**You flag and suggest. You do not rewrite automatically.**

---

## Usage

Invoked as `/docs-seo [file-path]`

If no path is given, ask which file to review.

If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop and say this skill only checks discoverability of published documentation pages — a skill file is never rendered as a webpage and has no search relevance to evaluate.

---

## Step 1: Read and Identify

Read the full file. Identify:
- The doc type (quickstart, guide, concept, reference, use-case)
- The primary task or concept the page covers
- The likely developer search query that would lead someone here

State both before running the checks. If you cannot identify a primary task or concept, that is itself a signal for Check 4.

---

## Check 1: Title Search Intent

**Goal:** The page title should match the phrase a developer would type when they have this problem.

### Rules by doc type

**Quickstart and guide** — titles must be task-oriented. A developer searching for help types a task, not a category label.

| ❌ Fails | ✅ Passes |
|---|---|
| "App Native Authentication" | "Sign In Without a Browser Redirect" |
| "Manage Applications" | "Create and Configure Applications" |
| "Flow Configuration" | "Configure a Sign-In Flow" |
| "Android Integration" | "Add Sign-In to an Android App" |

Failure patterns:
- Generic noun labels: "Overview", "Setup", "Configuration", "Introduction", "Management"
- Product-first framing when the product name adds no search value: "ThunderID Application Settings" (a developer in trouble doesn't start by typing the product name)
- Vague gerunds: "Getting Started" (getting started with *what*, from *where*, for *whom*?)

**Concept** — noun phrases are acceptable if they name the exact concept a developer would search for.

| ❌ Fails | ✅ Passes |
|---|---|
| "Flows" | "Authentication Flows" |
| "Tokens" | "Access Tokens and Refresh Tokens" |
| "Identity" | "Identity Providers in ThunderID" |

The test: would a developer searching for this concept type this exact phrase into Google?

**Reference** — noun phrases describing the subject are fine. "API Reference", "Configuration Options", "Flow Node Types" are all searchable.

**Use-case** — should name the scenario clearly enough that someone with that scenario recognizes it: "Multi-Tenant Authentication", "B2B SSO Setup", "Guest Checkout Without Registration."

### What to flag

Flag any title that:
- Is a generic label with no task or concept specificity
- Would match dozens of other products' pages unchanged
- Omits the technology name when the technology is what makes the page findable (e.g., a React quickstart that doesn't say "React")

For each flag: quote the current title, explain what search query it fails to capture, and suggest a rewrite.

---

## Check 2: Meta Description Quality

**Goal:** The `description` frontmatter field appears as the snippet under the page title in Google results. A developer scans it in under three seconds to decide whether to click.

`docs-check` already warns if the description is under 70 or over 200 characters, and fails if it starts with "This page/guide/document." Length is only a rough floor/ceiling, though — it's not a Docusaurus requirement or a Google ranking factor, just an approximation of search-snippet truncation. This check goes further and matters more: is the description *useful for a developer deciding to click*?

### What a good description does

- States the outcome the developer gets after following the page ("Add sign-in to a React app using the ThunderID JavaScript SDK in under 10 minutes.")
- Uses the terms the developer is searching for (the technology, the protocol, the task)
- Answers the implicit question: "why would I click this result instead of the others?"

### Failure patterns

- Restates the title in different words without adding information
- Describes the page ("This guide explains how to configure...") instead of the outcome
- Generic enough to apply to any auth product's equivalent page
- Passive and noun-heavy ("Configuration of redirect URIs for use in authorization flows")

### What to flag

For each description that fails: quote it, state what it is missing (outcome, technology, specificity), and suggest a rewrite or the type of change needed.

---

## Check 3: Heading Phrasing

**Goal:** H2 and H3 headings are indexed by Google and appear in featured snippets and "People also ask" results. Generic single-word headings miss these opportunities.

### Generic headings to flag

Flag any non-Stepper H2 that uses one of these generic labels (or a close variant):

`Overview`, `Introduction`, `Background`, `Summary`, `Notes`, `Details`, `Configuration` (alone, without a subject), `Setup` (alone), `Usage` (alone), `Options` (alone)

These headings are structurally fine but rank for nothing. A developer searching for "configure redirect URI ThunderID" will not find a heading called "Configuration."

**Exception — Stepper pages:** On pages with `<Stepper>`, all H2s are imperative step headings already enforced by `docs-review-style`. Do not flag these.

**Exception — "Prerequisites":** A structural heading, not a search target. Skip it.

### Good heading patterns

A heading earns its place in search when it names the specific subject:

| ❌ Generic | ✅ Specific |
|---|---|
| `## Configuration` | `## Configure the Redirect URI` |
| `## Overview` | `## How Authorization Code Flow Works in ThunderID` |
| `## Setup` | `## Set Up a Google Identity Provider` |
| `## Options` | `## Token Lifetime Configuration Options` |

### What to flag

For each generic heading: quote it with line number, explain what specific subject it should name, and suggest a rewrite.

---

## Check 4: Topic Focus

**Goal:** A page that covers too many unrelated developer tasks will not rank for any of them. Google rewards pages that clearly answer one question.

### How to evaluate

Read the H2 structure end-to-end. Ask: how many distinct developer tasks does this page serve?

A page serves one task if a developer could describe it in one sentence: "I want to add Google Sign-In to my ThunderID app." A page serves multiple tasks if it requires multiple sentences with "and also."

### Failure patterns

- A guide that creates, edits, deletes, and configures a resource all on one page — four separate developer intents
- A concept page that explains both the concept and provides a full how-to guide (concept bleed — also caught by `docs-review-style`)
- A quickstart that covers three different frameworks on one page

### What to flag

If the page has two or more H2 sections that serve completely different developer search intents, flag it. State:
- What the competing intents are
- Whether the page could be split, or whether one intent should become the page's clear primary focus

This is a judgment call. Flag confidently when the intents are clearly different; note uncertainty when they are related sub-tasks of one workflow.

---

## Output Format

```
Reviewing: docs/content/guides/guides/applications/manage-applications.mdx
Doc type: guide
Primary task: creating and configuring applications in the Console
Likely search query: "create application ThunderID", "configure OAuth application ThunderID"

SEO CHECK 1: TITLE SEARCH INTENT
  ⚠️  "Manage Applications" — noun label, not a task phrase. Developers searching for help type a task ("create ThunderID application", "configure OAuth settings ThunderID"), not a management category. Consider retitling to the primary task this page serves, or splitting the page by task.

SEO CHECK 2: META DESCRIPTION
  ✅  Describes the outcome and uses specific terms a developer would search for.

SEO CHECK 3: HEADING PHRASING
  ⚠️  line 45: "## Overview" — generic; ranks for nothing. Rewrite to name the specific concept (e.g., "## What ThunderID Applications Are").
  ✅  "## Create an Application" — task-oriented, matches developer query.
  ✅  "## Configure OAuth Settings" — specific and searchable.

SEO CHECK 4: TOPIC FOCUS
  ⚠️  Page covers five distinct developer tasks: creating, editing, deleting, OAuth config, and SAML config. These have separate search intents. Consider splitting into focused task pages, or confirm this is intentionally a reference hub (in which case Check 1's title should reflect that).

─────────────────────────────────────
0 failures · 3 warnings
(All findings are advisory — no hard gates.)
```

If all checks pass:
```
─────────────────────────────────────
✅  All checks passed — no SEO concerns found.
```

---

## Scope Boundary

| Check | This skill | Other skill |
|---|---|---|
| Title search intent | ✅ | |
| Meta description click-worthiness | ✅ | |
| Heading phrasing for discoverability | ✅ | |
| Topic focus for ranking | ✅ | |
| Description length and format | | `/docs-check` |
| Heading hierarchy and structure | | `/docs-check` |
| Writing quality, AI vocabulary, voice, terminology, formatting | | `/docs-review-style` |
| Technical accuracy | | `/docs-review-tech` |
