# ThunderID Agent Guides

Conventions that are too long to keep in an always-loaded `AGENTS.md` and too declarative to be a skill. Each file
below is a set of invariants for one area, plus the reasoning that makes them stick.

**This index exists because guides are hard to find by searching.** `.agent/` is a hidden directory, so `rg` skips it
unless you pass `--hidden`. If you know a convention exists but not where it lives, read this table rather than
grepping.

## How guides load

Each guide carries `paths:` frontmatter listing the file globs it applies to. Three consumers read them:

- **Claude Code** through the `.claude/rules` symlink, which points at this directory. A guide loads automatically when
  Claude reads a file matching its `paths:`.
- **CodeRabbit** through `knowledge_base.code_guidelines.filePatterns` in `.coderabbit.yaml`, which maps each guide to
  the source globs it should be applied to during review.
- **Any agent** following a routing table in the nearest `AGENTS.md`, which is the fallback when automatic loading does
  not fire.

Because a reviewer applies these rules too, write each one so it can be checked, not only followed. Most guides end with
a "Flagging this in review" section for that reason.

## The guides

| Guide | Applies when you are touching |
|---|---|
| `database.md` | Schema scripts under `backend/dbscripts/`, a `store.go`, or a `model.DBQuery` definition |
| `go-testing.md` | `backend/**/*_test.go` or the `tests/integration/` module |
| `e2e-page-objects.md` | The Playwright suite under `tests/e2e/` |
| `frontend-error-display.md` | Anywhere a mutation or query failure is surfaced to the user |
| `frontend-routing.md` | A route, or any `navigate` / `<Link>` / `<Route>` destination |
| `frontend-forms.md` | A form section using react-hook-form and zod |
| `frontend-edit-pages.md` | A `*EditPage.tsx` or an `edit-*` child section, and its reset-key contract |
| `frontend-package-build.md` | A `rolldown.config.js`, a new `frontend/packages/*`, or a page rendering Monaco |
| `oxygen-ui.md` | Any `.tsx` in `frontend/` or `docs/src/`: components, icons, styling |
| `docs-site.md` | Docusaurus machinery under `docs/src/`, config, plugins, or build scripts |
| `cross-os-paths.md` | Build tooling or a Node script that manipulates filesystem paths |

## Adding a guide

1. Give it `paths:` frontmatter. Without it the guide loads on **every** session at the same priority as project
   instructions, which is the opposite of the point.
2. Add a row to the table above, and a row to the routing table in the nearest `AGENTS.md`. A guide nothing routes to
   will not be read when automatic loading misses.
3. Add a `{files, applyTo}` entry in `.coderabbit.yaml` under `knowledge_base.code_guidelines.filePatterns`, so review
   applies it too. A bare glob string is scoped to its own directory and therefore applies to nothing reviewable.
4. Never restate a rule from a guide in a routing table. A routing row carries a trigger, a path, and a purpose clause,
   nothing more. Restating is how the previous structure drifted.

Nothing enforces these mechanically, so they are worth checking by hand when you add or move a guide. Steps 1 and 3
fail silently rather than loudly: a guide with no `paths:` loads on every session instead of on demand, and a guide with
no `applyTo` mapping is never applied during review.
