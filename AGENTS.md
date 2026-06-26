# Project Overview

ThunderID is a lightweight user and identity management product: a Go backend (`backend/`) and React frontend (`frontend/`) in a monorepo. It provides authentication and authorization via OAuth2/OIDC, flexible orchestration flows, and individual auth mechanisms (password, passwordless, social login).

- [ARCHITECTURE.md](ARCHITECTURE.md) — read only for cross-cutting changes
- Build and run: [Makefile](Makefile) and [README.md](README.md)
- Documentation at [docs/content](docs/content)

Login Gate consumes ThunderID's published JavaScript SDK packages — `@thunderid/react` and `@thunderid/react-router` — from the [javascript-sdks](https://github.com/thunder-id/javascript-sdks) repository. Clone that repository only if you are developing or debugging the SDK itself, or testing the product against unreleased SDK changes.

## Where to Look Next

Load only the guidance the task needs:

| Working on… | Read |
|---|---|
| Backend Go code | [backend/AGENTS.md](backend/AGENTS.md) |
| Frontend React code | [frontend/AGENTS.md](frontend/AGENTS.md) |
| Documentation | [docs/AGENTS.md](docs/AGENTS.md) |
| Database schema, queries, or stores | [.agent/skills/db/SKILL.md](.agent/skills/db/SKILL.md) |
| Browser automation / Console UI verification | [.agent/skills/console/SKILL.md](.agent/skills/console/SKILL.md) |

The `.agent/skills/` entries above are internal guidance for **developing** ThunderID. Consumer-facing setup and framework-integration skills (`setup-thunderid`, `integrate-*`) live in the separate ThunderID Skills repository (installable via `/plugin marketplace add thunder-id/skills`) and are not used when working in this repo.

## Search Hygiene

- Prefer `rg` over `grep`/`find`.
- `.gitignore` (which `rg` honors) already excludes build outputs, `node_modules`, `.claude/`, `/coverage`, `tests/e2e/distribution/`, `.turbo`, and generated API specs — do not search them.
- Do not search mirror worktrees under `.claude/worktrees/`.

## Validation Ladder

- **Inner loop**: run the smallest relevant checks for the area you touched. The scoped AGENTS files say which tests to run.
- **Pre-PR gate**: `make pr_checks` — the authoritative gate (verify_mocks → lint → format_check → unit/frontend/integration tests → builds).
- Note: `make test` is backend-only (unit + integration), not full-repo validation.

## General Rules

- Keep changes minimal and focused on the task requested. Do not refactor, "improve", or clean up surrounding code.
- Do not add comments, docstrings, or type annotations to code you did not change.
- Prefer editing existing files over creating new ones.
- Do not add new dependencies or modify CI/CD pipelines, GitHub Actions, or Makefiles without explicit approval.
- Do not over-engineer. No premature abstractions, no feature flags, no backwards-compatibility shims.
- Mocks are auto-generated via `make mockery`. Do not generate or modify mock files manually.
- Delete dead code cleanly. No `// removed` or `// deprecated` placeholder comments. No renaming unused variables to `_` prefixed names — remove them entirely unless required by an interface, callback, or framework signature.
- Do not create fallback tests with mock/hardcoded data when original tests fail. Fix the actual failing tests.
- Do not add error handling for scenarios that cannot happen.
- Write tests for new features and bug fixes (target 80%+ coverage).
- Ensure proper error handling and logging at appropriate layers — not everywhere, just where failures are expected and actionable.
- Ensure all identity-related code aligns with relevant RFC specifications.
- Declarative resource attributes use camelCase, matching the REST API. The `yaml` struct tag must use the same camelCase name as the field's `json` tag (for example `yaml:"ouId"`, not `yaml:"ou_id"`). This does not apply to non-declarative YAML such as `deployment.yaml` server config, or to `json` tags for protocol payloads (OAuth, DCR) that follow their own RFC conventions.

## Git and PR Conventions

- Adhere to [.github/pull_request_template.md](.github/pull_request_template.md).

### Commit Messages
- Use short imperative sentences without conventional commit prefixes (no `feat:`, `fix:`, etc.).
- Reference the related issue or pull request when applicable (e.g., `Refs #123` or `Fixes #123`).

### One Commit Per Pull Request
- Each PR must have a single commit. Only add a second commit when strictly necessary. Never leave intermediate or fixup commits in the PR.
