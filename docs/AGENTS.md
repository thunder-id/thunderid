---
title: AGENTS
description: AI agents should use this file when creating and reviewing documentation content for ThunderID. It points to the agent skills that scaffold, write, and review docs, rather than restating their rules here.
---

# ThunderID Documentation — Agent Instructions

Documentation content under `docs/content/` has a dedicated skill for every stage: scaffolding, writing, and review. **Invoke the matching skill instead of creating, editing, or reviewing a doc page by hand** — each one enforces the project's writing standards directly, so the rules live in one executable place, not duplicated in prose here. This applies even when the request doesn't name a skill: "write documentation for X," "document this feature," "add a section on Y," "review this page," and "does this doc meet standards" should all route to the matching skill below, not to a manual edit or review.

- `/docs-new-page` — creating a new page, or "write documentation for X" when no existing page covers the topic. Checks for an existing page on the same topic first, creates the file from the right template, and proposes where it belongs in the sidebar.
- `/docs-edit` — writing content into an existing page (a placeholder, a gap, or a new section). Verifies every technical claim against the codebase, existing docs, a standard spec, or a supplied draft before writing it.
- `/docs-check` — structural standards only: frontmatter, headings, links, Stepper config, sidebar registration.
- `/docs-review-style` — writing quality only: AI-sounding prose, tone, voice consistency, em/en dashes, and more.
- `/docs-review-tech` — technical accuracy only: protocol, API, SDK, config, and security claims verified against source.
- `/docs-review` — the full pre-merge check: runs all three checks above and combines them into one pass/fail report.
- `/docs-seo` — discoverability check for new pages or major rewrites.

See [docs/README.md](README.md) for the full contributor workflow with a worked example, and each skill's own `.agent/skills/<name>/SKILL.md` for its complete rules.
