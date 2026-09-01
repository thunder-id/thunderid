# Project Overview

ThunderID is a lightweight, open-source IAM stack: a Go backend (`backend/`) and React frontend (`frontend/`) in a monorepo. It provides authentication and authorization via OAuth2/OIDC, flexible orchestration flows, and individual auth mechanisms (password, passwordless, social login).

- [ARCHITECTURE.md](ARCHITECTURE.md) — read only for cross-cutting changes
- Build and run: [Makefile](Makefile) and [README.md](README.md)
- Documentation at [docs/content](docs/content)

## Where to Look Next

Guidance comes in three tiers. Load only what the task needs.

- **Routers** (`AGENTS.md`, one per area) carry orientation plus the rules that are always relevant in that area. Each
  has a `CLAUDE.md` beside it importing it, so it loads automatically when you touch files in that directory.
- **Guides** (`.agent/guides/*.md`) carry deep conventions for one topic and load only when they are relevant. They are
  reachable three ways: automatically via the `.claude/rules` symlink, via the routing table in the nearest `AGENTS.md`,
  or via the index at [.agent/guides/README.md](.agent/guides/README.md).
- **Skills** (`.agent/skills/*/SKILL.md`) are procedures you invoke, not conventions you follow.

| Trigger | Read | Tier |
|---|---|---|
| Any file under `backend/` | [backend/AGENTS.md](backend/AGENTS.md) | router |
| Any file under `frontend/` | [frontend/AGENTS.md](frontend/AGENTS.md) | router |
| Any file under `docs/` | [docs/AGENTS.md](docs/AGENTS.md) | router |
| Any file under `tests/` | [tests/AGENTS.md](tests/AGENTS.md) | router |
| Authoring `api/*.yaml` or `api/extensions/*.yaml` | [api/AGENTS.md](api/AGENTS.md) | router |
| Build tooling or a Node script that manipulates paths (`tools/**`, `docs/scripts/*.mjs`, `*.config.{ts,js}`) | [.agent/guides/cross-os-paths.md](.agent/guides/cross-os-paths.md) | guide |
| Any `.tsx` under `frontend/` or `docs/src/` | [.agent/guides/oxygen-ui.md](.agent/guides/oxygen-ui.md) | guide |
| Any documentation task: writing, editing, or reviewing a page | `docs` skill | skill |
| Driving the Console in a browser to verify UI | `console` skill | skill |
| You know a convention exists but not where it lives | [.agent/guides/README.md](.agent/guides/README.md) | index |

A routing row carries a trigger, a path, and a purpose clause, and never restates a rule from its target. Restating is
how the previous structure drifted: a copied list went stale and silently dropped an entry.

The `.agent/` tree is internal guidance for **developing** ThunderID. Consumer-facing setup and framework-integration
skills (`setup-thunderid`, `integrate-*`) live in the separate ThunderID Skills repository (installable via
`/plugin marketplace add thunder-id/skills`) and are not used when working in this repo.

## Search Hygiene

- `rg` honors `.gitignore`, which already excludes build outputs, `node_modules`, `/coverage`, `tests/e2e/distribution/`, `.turbo`, and generated API specs — don't search them.
- `rg` also skips `.agent/` and `.claude/` because they are hidden directories, not because of `.gitignore`. To search agent guidance itself you need `rg --hidden`. Add `-L` only if you want to follow the `.claude/skills` and `.claude/rules` symlinks, which otherwise report every hit twice.

## Validation Ladder

- **Inner loop**: run the smallest relevant checks for the area you touched. The scoped AGENTS files say which tests to run.
- **Pre-PR gate**: `make pr_checks` (verify_mocks → lint → format_check → unit/frontend/integration tests → builds).
- `pr_checks` does **not** cover everything. Its `lint` is `lint_backend` + `lint_frontend` only, so run these yourself when they apply:
  - Changed anything under `docs/` → also `make lint_docs` and `make build_docs`.
  - Changed Console or Gate UI → also `make test_e2e`.
- Two more traps: `make test` is backend-only (unit + integration), and root `pnpm lint` / `pnpm test` are filtered with `--filter=!./tests/**` (`test` also excludes `./tools/**`), so they silently skip the e2e suite. Run those from `tests/e2e`.

## Product Name Rules

- Always use `ThunderID` (or the appropriate template placeholder for the file type). Never use the bare word `thunder`, `Thunder`, or `THUNDER` as a short form of the product name.
- PRs that introduce bare `thunder`/`Thunder`/`THUNDER` (not part of `thunderid`, `ThunderID`, or `THUNDERID`) must not be merged until corrected.
- Exceptions: import paths and package names (e.g. `@thunderid/...`), and code identifiers where `thunder` is a structural prefix immediately followed by `id` or `-id` in any casing. The `-id` form matters: the GitHub org is `thunder-id` and the Go module is `github.com/thunder-id/thunderid`, so a literal reading without it flags the repo's own module path.
- Never hardcode the product name as a string literal in Go or TypeScript. Source it from config: `useConfig()` in TS (`const {productName} = useConfig()`), the injected server config in Go, or `<ProductName />` in JSX. Hardcoded brand strings are what make the product un-rebrandable.

## Product Positioning

- Canonical category descriptor (prose): **open-source IAM stack**. Use it wherever the product is defined in prose (READMEs, docs, Helm charts). Do not reintroduce "IAM engine/platform/server", "identity management suite/system/product/platform/service", or "Identity Provider" as the product category.
- Audience triplet: **humans, AI agents, and machines**, in that order. Do not use "workloads" or "resources".
- The four product pillars, in order: **Agent-native Identity**, **Post-quantum-safe by Design**, **Decentralized Identity**, **Lightweight Runtime with GitOps Support**. Reuse these names (match each surface's casing convention).
- Exception: the docs marketing tagline **"Auth for Modern Apps and AI Agents"** (`docs/docusaurus.product.config.ts`) is a deliberate slogan, not the prose category descriptor; leave it as-is.

## General Rules

Every rule here has a mechanism in this repo that catches a violation. Model-default good behaviour is not repeated.

- **New source files need the licence header.** Two lines, current year, at the very top:
  ```
  // Copyright <YEAR> The ThunderID Authors
  // SPDX-License-Identifier: Apache-2.0
  ```
  Lint-enforced on the frontend by `@thunderid/eslint-plugin`'s `copyright-header` rule; convention-only on the backend, so nothing will remind you there.
- **Do not add dependencies, or change CI/CD pipelines, GitHub Actions, or Makefiles, without explicit approval.** Frontend dependency versions come from the `pnpm-workspace.yaml` catalog, not from per-package ranges.
- **Delete dead code cleanly.** No `// removed` or `// deprecated` placeholder comments, and no renaming an unused variable to a `_` prefix. Remove it, unless an interface, callback, or framework signature requires the parameter. Unreferenced doc pages are tracked in `.orphan-allowlist`.
- **Never write a fallback test with mock or hardcoded data because the real test fails.** Fix the failing test. This is exactly the failure mode `make verify_mocks` exists to catch.
- **Write tests for new features and bug fixes.** The binding coverage numbers are per-package and live in `.github/backend-coverage-thresholds.yml` and `codecov.yml`; read them there rather than assuming a single repo-wide target.
- **No em dashes or `--` in copy or UI strings**, including i18n locale files and Go i18n defaults. Prefer a comma, a period, or rephrasing. Enforced by `.vale/styles/ThunderID/EmDashes.yml`.
- **No backwards-compatibility shims.** ThunderID is pre-1.0; change the thing rather than adding a compatibility layer around it.

## Git and PR Conventions

- Adhere to [.github/pull_request_template.md](.github/pull_request_template.md).
- Keep the diff to what the task asked for. Drive-by refactors of surrounding code make a single-commit PR unreviewable.

### Commit Messages
- Use short imperative sentences without conventional commit prefixes (no `feat:`, `fix:`, etc.).
- Reference the related issue or pull request when applicable (e.g., `Refs #123` or `Fixes #123`).

### One Commit Per Pull Request
- Each PR must have a single commit. Never leave intermediate or fixup commits in the PR.
