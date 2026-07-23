---
name: docs
description: Handles every ThunderID documentation task in one skill: scaffolding a new page, writing content into an existing page, checking structural standards, reviewing writing quality/tone/AI-vocabulary, verifying technical accuracy, reviewing API documentation (OpenAPI specs and SDK reference pages) for consistency and accuracy, checking SEO/discoverability, or running the full pre-merge review. Use whenever asked to create, write, add, or fill in documentation; check, lint, or verify a doc page's standards; review a doc's style, tone, or quality; verify a doc's technical accuracy; review an OpenAPI spec or SDK API reference page; check a doc's SEO; or do a full/complete review before merging a doc PR.
allowed-tools: Read Write Edit Bash WebFetch WebSearch
---

# ThunderID Documentation Skill

One skill for every documentation task, split into reference files so only the relevant one loads into context. This file is a dispatch table, not the rules themselves — the actual instructions live in the reference file(s) below.

If the target path is a `SKILL.md` under `.agent/skills/` or `.claude/skills/`, stop for every action below: these are agent instructions, not documentation, and none of these rules apply.

## Step 1: Match the request to a reference file

### Explicit form

If invoked as `/docs <action> [file-path]`, or the request plainly names one of these action words, map directly with no further interpretation:

| action | reference |
|---|---|
| `new-page` | `new-page.md` |
| `edit` | `edit.md` |
| `check` | `check.md` |
| `style` | `style.md` |
| `tech` | `tech.md` |
| `api` | `api.md` |
| `seo` | `seo.md` |
| `review` | `review.md` |

### Natural-language form

Otherwise, match the request's intent:

| The user is asking to... | Read |
|---|---|
| Create a new page, or "write docs for X" when nothing covers the topic yet | `new-page.md` |
| Write or fill in content on an existing page (a placeholder, a gap, a new section) | `edit.md` |
| Check, lint, or verify structural standards (frontmatter, headings, links, sidebar registration, etc.) | `check.md` |
| Review writing quality, tone, AI-sounding prose, voice consistency | `style.md` |
| Verify technical accuracy (protocol/API/SDK/config/security claims) | `tech.md` |
| Review an OpenAPI spec (`api/*.yaml`) or an SDK API reference page (`docs/content/sdks/*/apis/**`) for consistency/accuracy | `api.md` |
| Check SEO / discoverability | `seo.md` |
| Run the full pre-merge review (structure + style + tech together) | `review.md` |
| Review my changes / review the diff / review before I open a PR, no file named | `review.md` (diff mode — reads every changed file, including uncommitted, instead of one named file) |

Default to `review.md` if the request just says "review this doc" without naming a dimension or an action word.

### Path-based override

If the target path matches `api/*.yaml` or `docs/content/sdks/*/apis/**`, prefer `api.md` over `check.md`/`tech.md` even for a generic "check this file" or "verify this" request that doesn't name API docs explicitly — those paths are specifically what `api.md` is for, and it covers more ground for them than the generic references do.

Read the matched file now and follow it exactly. When a reference file mentions another dimension by name (e.g. "the check reference" or "the style reference"), that means read the corresponding file in this same directory — there's no longer a separate slash command per dimension, just this one skill.

## Step 2: State which reference(s) you're following

Before doing the work, say in one line which file(s) you read for this request (e.g. "Following check.md" or "Following review.md, which runs check.md, style.md, and tech.md in sequence"). This keeps the dispatch visible instead of silent.
