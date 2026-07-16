---
name: docs-seo
description: SEO review for ThunderID docs: checks title search-intent match, meta description click-worthiness, H2 phrasing, and topic focus. Use whenever asked to check a doc's discoverability, searchability, or SEO, especially for new pages or major rewrites. Not writing quality or tech accuracy (see docs-review-style/docs-review-tech).
allowed-tools: Read Bash
---

# ThunderID Docs — SEO Review

Reviewing a page for search discoverability: would a developer searching for this topic find it, click it, and land in the right place?

Not keyword stuffing or term density — ThunderID docs already use correct technical language. This checks whether the page is *structured and titled* to match how developers phrase their searches.

All findings are warnings — no hard gates, SEO is advisory. **You flag and suggest; you do not rewrite automatically.**

---

## Usage

Invoked as `/docs-seo [file-path]`. If no path is given, ask which file. If the path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop: it's never rendered as a webpage, so it has no search relevance to evaluate.

---

## Step 1: Read and Identify

Read the full file. Identify the doc type, the primary task/concept it covers, and the likely developer search query. State all three before running checks. If you can't identify a primary task/concept, that itself is a signal for Check 4.

---

## Check 1: Title Search Intent

**Goal:** the title should match the phrase a developer would type when they have this problem.

**Quickstart/guide** — must be task-oriented; a developer in trouble types a task, not a category label.

| ❌ Fails | ✅ Passes |
|---|---|
| "App Native Authentication" | "Sign In Without a Browser Redirect" |
| "Manage Applications" | "Create and Configure Applications" |
| "Flow Configuration" | "Configure a Sign-In Flow" |
| "Android Integration" | "Add Sign-In to an Android App" |

Failure patterns: generic noun labels ("Overview", "Setup", "Configuration", "Introduction", "Management"); product-first framing that adds no search value ("ThunderID Application Settings"); vague gerunds ("Getting Started" — with what, from where, for whom?).

**Concept** — noun phrases are fine if they name the exact concept a developer would search for ("Flows" fails, "Authentication Flows" passes). Test: would they type this exact phrase into Google?

**Reference** — noun phrases describing the subject are fine ("API Reference", "Configuration Options").

**Use-case** — should name the scenario clearly enough to be recognized: "Multi-Tenant Authentication", "B2B SSO Setup", "Guest Checkout Without Registration."

Flag any title that's a generic label with no specificity, would match dozens of other products' pages unchanged, or omits the technology name when that's what makes the page findable (a React quickstart that doesn't say "React"). For each: quote the title, explain the search query it misses, suggest a rewrite.

---

## Check 2: Meta Description Quality

**Goal:** the `description` frontmatter appears as the Google snippet under the title; a developer scans it in under 3 seconds to decide whether to click.

`docs-check` already warns on length (under 70/over 200 chars, a rough heuristic not a ranking factor) and fails on "This page/guide/document" openers. This check goes further: is it *useful for deciding to click*?

A good description states the outcome ("Add sign-in to a React app using the ThunderID JavaScript SDK in under 10 minutes"), uses the terms being searched for (technology, protocol, task), and answers "why click this result instead of others?"

Failure patterns: restates the title without adding information; describes the page ("This guide explains how to configure...") instead of the outcome; generic enough for any auth product's equivalent page; passive and noun-heavy ("Configuration of redirect URIs for use in authorization flows").

For each failure: quote it, state what's missing (outcome, technology, specificity), suggest a rewrite or the type of change needed.

---

## Check 3: Heading Phrasing

**Goal:** H2/H3 headings get indexed and can surface in featured snippets and "People also ask." Generic single-word headings miss that.

Flag any non-Stepper H2 using a generic label or close variant: `Overview`, `Introduction`, `Background`, `Summary`, `Notes`, `Details`, `Configuration`/`Setup`/`Usage`/`Options` (alone, no subject). Structurally fine, ranks for nothing — "configure redirect URI ThunderID" won't match a heading called "Configuration."

**Exceptions**: Stepper pages (H2s are already imperative step headings enforced by `docs-review-style` — don't flag); "Prerequisites" (structural, not a search target).

| ❌ Generic | ✅ Specific |
|---|---|
| `## Configuration` | `## Configure the Redirect URI` |
| `## Overview` | `## How Authorization Code Flow Works in ThunderID` |
| `## Setup` | `## Set Up a Google Identity Provider` |
| `## Options` | `## Token Lifetime Configuration Options` |

For each generic heading: quote it with line number, explain what subject it should name, suggest a rewrite.

---

## Check 4: Topic Focus

**Goal:** a page covering too many unrelated tasks won't rank for any of them — Google rewards pages that clearly answer one question.

Read the H2 structure end-to-end. One task = describable in one sentence ("I want to add Google Sign-In to my ThunderID app"). Multiple tasks = needs "and also."

Failure patterns: a guide that creates, edits, deletes, and configures a resource all on one page (four intents); a concept page that also provides a full how-to (concept bleed, also caught by `docs-review-style`); a quickstart covering three frameworks on one page.

If 2+ H2 sections serve completely different search intents, flag it: state the competing intents, and whether the page should split or one intent should become the clear primary focus. Judgment call — flag confidently when intents are clearly different, note uncertainty when they're related sub-tasks of one workflow.

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
