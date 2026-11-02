---
title: AGENTS
description: AI agents should use this file when creating and reviewing documentation content for ThunderID. It points to the docs skill that scaffolds, writes, and reviews docs, rather than restating its rules here.
---

# ThunderID Documentation — Agent Instructions

Documentation content under `docs/content/` and `docs/community/` is handled by one skill, `docs` (`.agent/skills/docs/SKILL.md`), covering every stage: scaffolding, writing, and review. **Invoke it instead of creating, editing, or reviewing a doc page by hand** — it enforces the project's writing standards directly, so the rules live in one executable place, not duplicated in prose here. This applies even when the request doesn't name it: "write documentation for X," "document this feature," "add a section on Y," "review this page," and "does this doc meet standards" should all route to it, not to a manual edit or review.

The skill's `SKILL.md` is a dispatch table over its reference files, each covering one stage:

- `new-page.md` — creating a new page, or "write documentation for X" when no existing page covers the topic. Checks for an existing page on the same topic first, creates the file from the right template, and proposes where it belongs in the sidebar.
- `edit.md` — writing content into an existing page (a placeholder, a gap, or a new section). Verifies every technical claim against the codebase, existing docs, a standard spec, or a supplied draft before writing it.
- `check.md` — structural standards only: frontmatter, headings, links, Stepper config, sidebar registration.
- `style.md` — writing quality only: AI-sounding prose, tone, voice consistency, em/en dashes, and more.
- `tech.md` — technical accuracy only: protocol, API, SDK, config, and security claims verified against source.
- `api.md` — API documentation specifically: OpenAPI specs (`api/*.yaml`) verified against the Go backend's registered routes, and SDK reference pages (`docs/content/sdks/*/apis/**`) verified against build artifacts when available. Technical accuracy hard-gates here, same as `tech.md`.
- `review.md` — the full pre-merge check: runs `check.md`, `style.md`, `tech.md`, and (for API-reference paths) `api.md`, and combines them into one pass/fail report.
- `seo.md` — discoverability check for new pages or major rewrites.
- `use-case.md`, for structuring or auditing a use-case section (`docs/content/use-cases/<pattern>/`): page order, diagram design, jargon literacy for readers unfamiliar with IAM or ThunderID, and a scoring rubric. Use for creating, restructuring, or reviewing B2C, B2B, AI Agents, or a new use-case pattern.

See [docs/README.md](README.md) for the full contributor workflow with a worked example, and `.agent/skills/docs/` for each reference file's complete rules.

## Versioned Fetches in `docs/src/` Components

A component rendered inside actual doc content (an `.mdx` page under `docs/content/` or `docs/versioned_docs/`) that builds a URL to `fetch()` a versioned static asset (anything mirrored per-version under `static/docs/<versionPath>/...` or `static/api/<versionPath>/...`) must derive `versionPath` from `useDocsVersion()` (`@docusaurus/plugin-content-docs/client`), mapping `version === 'current' ? 'next' : version`. This is the same pattern already used in `docs/src/components/ApiVersionReference.tsx`.

Do not hardcode a `/docs/next/...` literal and rewrite it with `useDocsUrl()` (`@site/src/hooks/useDocsUrl`). That hook exists for version-*less* contexts (footer links, homepage, custom pages) that fall back through the site's global active/preferred/latest version state; a component embedded in a specific doc page already has its own authoritative version and should read it directly instead. `useDocsUrl()` remains the correct tool for rewriting `<Link>`/`href` navigation targets, since this rule only applies to runtime `fetch()` URLs. Enforced in `.coderabbit.yaml` under the `docs/**` path instructions.
