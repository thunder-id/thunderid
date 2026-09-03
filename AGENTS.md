# Project Overview

ThunderID is a lightweight, open-source IAM stack: a Go backend (`backend/`) and React frontend (`frontend/`) in a monorepo. It provides authentication and authorization via OAuth2/OIDC, flexible orchestration flows, and individual auth mechanisms (password, passwordless, social login).

- [ARCHITECTURE.md](ARCHITECTURE.md) — read only for cross-cutting changes
- Build and run: [Makefile](Makefile) and [README.md](README.md)
- Documentation at [docs/content](docs/content)

## Where to Look Next

Load only the guidance the task needs:

| Working on… | Read |
|---|---|
| Backend Go code | [backend/AGENTS.md](backend/AGENTS.md) |
| Frontend React code | [frontend/AGENTS.md](frontend/AGENTS.md) |
| Documentation | [docs/AGENTS.md](docs/AGENTS.md) |
| Database schema, queries, or stores | [.agent/skills/db/SKILL.md](.agent/skills/db/SKILL.md) |
| Browser automation / Console UI verification | [.agent/skills/console/SKILL.md](.agent/skills/console/SKILL.md) |
| Resolving a pnpm/npm security advisory | [.agent/skills/fix-npm-vulnerability/SKILL.md](.agent/skills/fix-npm-vulnerability/SKILL.md) |
| Auditing or pruning existing pnpm overrides | [.agent/skills/prune-npm-overrides/SKILL.md](.agent/skills/prune-npm-overrides/SKILL.md) |

The `.agent/skills/` entries above are internal guidance for **developing** ThunderID. Consumer-facing setup and framework-integration skills (`setup-thunderid`, `integrate-*`) live in the separate ThunderID Skills repository (installable via `/plugin marketplace add thunder-id/skills`) and are not used when working in this repo.

## Search Hygiene

- `rg` honors `.gitignore`, which already excludes build outputs, `node_modules`, `.claude/`, `/coverage`, `tests/e2e/distribution/`, `.turbo`, and generated API specs — don't search them.
- Do not search mirror worktrees under `.claude/worktrees/` (they duplicate the repo and double your results).

## Validation Ladder

- **Inner loop**: run the smallest relevant checks for the area you touched. The scoped AGENTS files say which tests to run.
- **Pre-PR gate**: `make pr_checks` — the authoritative gate (verify_mocks → lint → format_check → tool lint and tests → unit/frontend/integration tests → builds).
- The CLI e2e suite (`make tools_test_cli_e2e`) is not part of that gate: it downloads a real release and boots a server. CI runs it in `pr-builder.yml`, behind the `trigger-pr-builder` label, on every pull request.
- Per-tool tasks are namespaced `tools_*` (for example `make tools_lint_cli`, `make tools_test_i18n_extractor`); `tools_build`, `tools_test` and `tools_lint` run every tool.
- Note: `make test` is backend-only (unit + integration), not full-repo validation.

## Product Name Rules

- Always use `ThunderID` (or the appropriate template placeholder for the file type). Never use the bare word `thunder`, `Thunder`, or `THUNDER` as a short form of the product name.
- PRs that introduce bare `thunder`/`Thunder`/`THUNDER` (not part of `thunderid`, `ThunderID`, or `THUNDERID`) must not be merged until corrected.
- Exceptions: import paths/package names (e.g., `@thunderid/...`) and code identifiers where `thunder` is a structural prefix immediately followed by `id` in any casing are allowed.

## Product Positioning

- Canonical category descriptor (prose): **open-source IAM stack**. Use it wherever the product is defined in prose (READMEs, docs, Helm charts). Do not reintroduce "IAM engine/platform/server", "identity management suite/system/product/platform/service", or "Identity Provider" as the product category.
- Audience triplet: **humans, AI agents, and machines**, in that order. Do not use "workloads" or "resources".
- The four product pillars, in order: **Agent-native Identity**, **Post-quantum-safe by Design**, **Decentralized Identity**, **Lightweight Runtime with GitOps Support**. Reuse these names (match each surface's casing convention).
- Exception: the docs marketing tagline **"Auth for Modern Apps and AI Agents"** (`docs/docusaurus.product.config.ts`) is a deliberate slogan, not the prose category descriptor; leave it as-is.

## General Rules

- Keep changes minimal and focused on the task requested. Do not refactor, "improve", or clean up surrounding code.
- Do not add comments, docstrings, or type annotations to code you did not change.
- Prefer editing existing files over creating new ones.
- Do not add new dependencies or modify CI/CD pipelines, GitHub Actions, or Makefiles without explicit approval.
- Do not over-engineer. No premature abstractions, no feature flags, no backwards-compatibility shims.
- Delete dead code cleanly. No `// removed` or `// deprecated` placeholder comments. No renaming unused variables to `_` prefixed names — remove them entirely unless required by an interface, callback, or framework signature.
- Do not create fallback tests with mock/hardcoded data when original tests fail. Fix the actual failing tests.
- Write tests for new features and bug fixes (target 80%+ coverage).
- Add error handling and logging only where failures are expected and actionable — not for scenarios that cannot happen, and not everywhere.
- Do not use em dashes (—) or double hyphens (`--`) in copy or UI strings (e.g. i18n locale files). Prefer a comma, period, or rephrasing instead.

## Git and PR Conventions

- Adhere to [.github/pull_request_template.md](.github/pull_request_template.md).

### Commit Messages
- Use short imperative sentences without conventional commit prefixes (no `feat:`, `fix:`, etc.).
- Reference the related issue or pull request when applicable (e.g., `Refs #123` or `Fixes #123`).

### One Commit Per Pull Request
- Each PR must have a single commit. Never leave intermediate or fixup commits in the PR.
