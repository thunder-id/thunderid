# Backend — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md). The backend is a Go application under `backend/` (module `github.com/thunder-id/thunderid`): domain packages under `backend/internal/`, entry points under `backend/cmd/`, public packages under `backend/pkg/`.

For the canonical, deeper reference (package/file layout, export rules, logging), see the [Backend Development overview](../docs/content/community/contributing/contributing-code/backend-development/overview.mdx). Two sibling pages carry rules not repeated here: [Observability](../docs/content/community/contributing/contributing-code/backend-development/observability.mdx) (injecting the observability service, publishing events, event anatomy) and [Debugging](../docs/content/community/contributing/contributing-code/debugging.mdx).

## Always

- **Changing any interface means regenerating mocks**: run `make mockery`. The CI job `verify-mocks` (which runs the `verify_mocks` target) fails the build if mocks are out of sync. Never hand-edit a mock file.

## Conventions

- Ensure all identity-related code aligns with relevant RFC specifications.
- Declarative resource attributes use camelCase, matching the REST API. The `yaml` struct tag must use the same camelCase name as the field's `json` tag (for example `yaml:"ouId"`, not `yaml:"ou_id"`). This does not apply to non-declarative YAML such as `deployment.yaml` server config, or to `json` tags for protocol payloads (OAuth, DCR) that follow their own RFC conventions.
- Logging: use the `log` package from `internal/system` and pass the request `context.Context` first so entries carry the trace ID; avoid PII. See the overview for the full conventions (non-request contexts, `MaskString`).
- Cryptographic operations (encrypt/decrypt/sign/verify) must go through the injected `providers.RuntimeCryptoProvider`, not `internal/system/cryptolib` directly. `cryptolib` is a low-level primitives package meant to be wrapped by the default key manager provider (`internal/system/kmprovider/defaultkm`); other packages should depend on the `RuntimeCryptoProvider` interface so alternate key manager providers stay swappable.
- JWE "alg"/"enc" validation (e.g. checking a configured algorithm is supported) must use `jwe.JWEServiceInterface`'s `SupportedKeyEncryptionAlgorithms()` / `SupportedContentEncryptionAlgorithms()` methods, not ad-hoc calls to the crypto provider or hardcoded algorithm lists.

## Test Selection

- **Inner loop**:
  - service / store / API handler change → `make test_unit` first.
  - DB or API-contract change → also add `make test_integration` (filter with `RUN="TestName"` or `PACKAGE="pkg/path"`).
- **Pre-PR gate**: `make pr_checks`, per the root [AGENTS.md](../AGENTS.md). Running `make test_unit` + `make test_integration` alone is the inner loop, not the gate: it skips lint, format_check, the frontend tests, and the builds.

## Guides

| Trigger | Read |
|---|---|
| `store.go`, anything under `dbscripts/`, or a `model.DBQuery` definition | [.agent/guides/database.md](../.agent/guides/database.md) |
| `*_test.go`, or anything under `tests/integration/` | [.agent/guides/go-testing.md](../.agent/guides/go-testing.md) |
| Authoring or extending an OpenAPI spec | [api/AGENTS.md](../api/AGENTS.md) |

Before your first edit, read every guide whose trigger matches a file you are about to change, and state in one line which ones you loaded. If none match, say so.
