---
title: AGENTS
description: AI agents should use this file when creating and reviewing documentation content for ThunderID. It points to the agent skills that scaffold, write, and review docs, rather than restating their rules here.
---

# ThunderID Documentation — Agent Instructions

When working on documentation content under `docs/content/`, use these skills instead of writing or reviewing by hand. Each one enforces the project's writing standards directly, so the rules live in one executable place, not duplicated in prose here.

- `/docs-new-page` — scaffold a new page. Checks for an existing page on the same topic first, creates the file from the right template, and proposes where it belongs in the sidebar.
- `/docs-edit` — write content into a page (a placeholder, a gap, or a new section). Verifies every technical claim against the codebase, existing docs, a standard spec, or a supplied draft before writing it.
- `/docs-review` — the full pre-merge check: structure, writing quality, and technical accuracy in one command.
- `/docs-seo` — discoverability check for new pages or major rewrites.

See [docs/README.md](README.md) for the full contributor workflow with a worked example, and each skill's own `.agent/skills/<name>/SKILL.md` for its complete rules.
