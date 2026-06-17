# Project Overview

ThunderID is a lightweight user and identity management product. Go backend + React frontend in a monorepo. It provides authentication and authorization via OAuth2/OIDC, flexible orchestration flows, and individual auth mechanisms (password, passwordless, social login).

- [ARCHITECTURE.md](ARCHITECTURE.md)
- For build and running - [Makefile](Makefile) and [README.md](README.md)
- Documentation at [docs/content](docs/content)

Login Gate leverages v2 of the [ThunderID JavaScript SDK](https://github.com/thunder-id/thunderid/tree/main/sdks/javascript), consumed via its published package in typical setups.
Clone the SDK repository only if you are developing or debugging the SDK itself, or testing the product against unreleased SDK changes.

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
- Use `make lint` and `make test` to verify code quality and correctness before committing.

## Git and PR Conventions

- Adhere to .github/pull_request_template.md

### Commit Messages
- Use short imperative sentences without conventional commit prefixes (no `feat:`, `fix:`, etc.).
- Reference the related issue or pull request when applicable (e.g., `Refs #123` or `Fixes #123`).

### One Commit Per Pull Request
- Each PR must have a single commit. Only add a second commit when strictly necessary. Never leave intermediate or fixup commits in the PR.

## Agent Skills

- [Console Navigator](.agent/skills/console/SKILL.md) — Browse and interact with the Console UI using `playwright-cli`. Use when asked to navigate the console, test UI changes, or create/edit resources through the browser.
- [Database](.agent/skills/db/SKILL.md) — Database schema design principles and query conventions. Use for any database-related work.
- [Fix npm Vulnerability](.agent/skills/fix-npm-vulnerability/SKILL.md) — Resolve a pnpm/npm security advisory. Use when `pnpm audit` or Dependabot surfaces a security advisory. Tries to update the head dependency first; falls back to a scoped override with a tracking GitHub issue.

## Contributing Guidelines

- [`docs/content/community/contributing/contributing-code/backend-development/overview.mdx`](docs/content/community/contributing/contributing-code/backend-development/overview.mdx) — Go backend: package structure, database patterns, error handling, service initialization, transactions, testing
- [`docs/content/community/contributing/contributing-code/frontend-development/overview.mdx`](docs/content/community/contributing/contributing-code/frontend-development/overview.mdx) — React/TypeScript: component patterns, testing, linting
- [`docs/AGENTS.md`](/docs/AGENTS.md) — Documentation authoring standards

# Agent Guidance Index

Agent skills live under `.agent/skills/`.

- Database schema and query conventions: `.agent/skills/db/SKILL.md`

For any database-related work, follow `.agent/skills/db/SKILL.md`.
