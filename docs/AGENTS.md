# ThunderID Documentation — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md).

## Route Every Documentation Task to the `docs` Skill

Documentation content under `docs/content/` is handled by one skill, `docs` (`.agent/skills/docs/SKILL.md`), covering
every stage: scaffolding, writing, and review. **Invoke it instead of creating, editing, or reviewing a doc page by
hand.** This applies even when the request doesn't name it: "write documentation for X", "document this feature", "add a
section on Y", "review this page", and "does this doc meet standards" all route to it.

`SKILL.md` is the dispatch table. It is the only place the stage-to-reference-file mapping lives, so read it there
rather than trusting a copy: earlier copies of that table in this file and in [README.md](README.md) both went stale and
silently omitted a reference file that had been added to the skill.

See [README.md](README.md) for the contributor-facing workflow with a worked example.

## Validation

- `make lint_docs` runs Vale plus the structural checks in `scripts/docs-lint.sh` (frontmatter, heading hierarchy,
  Stepper config, links, `<ProductName />` usage, sidebar-orphan check against `.orphan-allowlist`).
- `make build_docs` catches broken links and MDX compile errors that linting alone does not.
- Neither is part of `make pr_checks`, so run both yourself when you change anything under `docs/`.

## Guides

| Trigger | Read |
|---|---|
| `docs/src/**`, `docs/docusaurus*.ts`, or `docs/src/css/custom.css` | [.agent/guides/docs-site.md](../.agent/guides/docs-site.md) |
| Any `.tsx` under `docs/src/` | [.agent/guides/oxygen-ui.md](../.agent/guides/oxygen-ui.md) |

Before your first edit, read every guide whose trigger matches a file you are about to change, and state in one line
which ones you loaded. If none match, say so.
