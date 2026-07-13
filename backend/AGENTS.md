# Backend — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md). The backend is a Go application under `backend/` (module `github.com/thunder-id/thunderid`): domain packages under `backend/internal/`, entry points under `backend/cmd/`, public packages under `backend/pkg/`.

For the canonical, deeper reference (flat package/file layout, export rules, logging), see the [Backend Development overview](../docs/content/community/contributing/contributing-code/backend-development/overview.mdx).

## When to Load Other Guides

- Touching `store.go`, schema, or queries → read [.agent/skills/db/SKILL.md](../.agent/skills/db/SKILL.md).
- Changing any interface → regenerate mocks with `make mockery`. CI's `verify_mocks` job fails if mocks are out of sync. Never hand-edit mock files.

## Conventions

- Ensure all identity-related code aligns with relevant RFC specifications.
- Declarative resource attributes use camelCase, matching the REST API. The `yaml` struct tag must use the same camelCase name as the field's `json` tag (for example `yaml:"ouId"`, not `yaml:"ou_id"`). This does not apply to non-declarative YAML such as `deployment.yaml` server config, or to `json` tags for protocol payloads (OAuth, DCR) that follow their own RFC conventions.
- Logging: use the `log` package from `internal/system` and pass the request `context.Context` first so entries carry the trace ID; avoid PII. See the overview for the full conventions (non-request contexts, `MaskString`).

## Test Selection

- **Inner loop**:
  - service / store / API handler change → `make test_unit` first.
  - DB or API-contract change → also add `make test_integration` (filter with `RUN="TestName"` or `PACKAGE="pkg/path"`).
- **Final gate**: `make test_unit` + `make test_integration`, or `make pr_checks` before opening a PR.
