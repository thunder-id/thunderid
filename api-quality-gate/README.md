# ThunderID API quality gate (Plan A)

Executable enforcement of API production-completeness. The design goal is to
**shrink human judgment to its irreducible core and make even that core
merge-blocking** — every rule that can be mechanized becomes a required status
check the merge cannot proceed without.

This directory is a **self-contained npm package**. It lives here (not at the
repo root) because the root is a pnpm monorepo; a second npm `package.json` at
the root would collide with pnpm. All commands below run from `api-quality-gate/`.

## What enforces what

| Failure class | Control | File | CI job |
|---|---|---|---|
| Collection endpoint with no pagination/filter/sort | Spectral rule | `.spectral.yaml` + `functions/requiredCollectionQueryParams.js` | `api-lint` |
| Inconsistent / missing error model | Spectral rules | `requiredErrorResponses.js`, `errorsUseProblemJson.js` | `api-lint` |
| Non-idempotent creates | Spectral rule | `writeHasIdempotency.js` | `api-lint` |
| Spec claims a feature the running service doesn't deliver | Contract tests (runtime) | `tests/integration/contract/` | `contract-tests` |
| Handler drifts from spec | Codegen drift check | `scripts/check-codegen-drift.sh` | `codegen-drift` |
| Backward-incompatible change | oasdiff | `.github/workflows/api-quality-gate.yml` | `breaking-changes` |
| New endpoint ships with no test | Coverage meta-check | `scripts/check-coverage.mjs` | `coverage-meta` |
| Resource model / "is this the right design" | **Human review, made blocking** | `.github/CODEOWNERS` on the spec | — |
| Someone weakens a rule or a waiver | Exemption governance + CODEOWNERS | `governance/exemptions.*`, `validate-exemptions.mjs` | `exemptions` |
| Someone weakens branch protection itself | Scheduled drift assertion | `scripts/check-branch-protection.sh`, `governance-drift.yml` | (scheduled) |

Spec-lint proves the YAML *claims* completeness; contract tests prove the
running service *delivers* it. You need both.

## Current scope in this repo

- **Enforced (merge-blocking) spec:** `api/group.yaml` — a pilot. ThunderID's
  API is 23 independent per-resource specs under `api/`; the gate is rolled out
  one resource at a time. The full-surface audit lives in the PR that introduced
  the gate.
- **Run model:** the Spectral ruleset resolves `overrides` globs relative to its
  own directory, so the pilot spec (repo-root `api/group.yaml`) is referenced
  from here as `../api/group.yaml`.
- **`codegen-drift` is a deliberate labelled no-op:** ThunderID's HTTP layer is
  hand-written (routes in `backend/internal/*/init.go`), so there is no
  OpenAPI→server codegen yet. `make generate` is a matching no-op. See
  `TODO(codegen)` in the workflow.
- **`contract-tests` uses Postgres only** (Redis is an alternative store, not
  required infra), booting the built distribution via the `tests/integration`
  harness.

## Verify it yourself (don't trust the ruleset blind)

```bash
cd api-quality-gate
nvm use              # any Node matching engines (^18.18 || >=20.17); repo .nvmrc is 24
npm ci
npm run test:rules    # bad sample must trip rules; good sample must pass clean
node scripts/validate-exemptions.mjs
npx spectral lint ../api/group.yaml -r .spectral.effective.yaml --fail-severity=warn
CONTRACT_DIR=../tests/integration/contract node scripts/check-coverage.mjs ../api/group.yaml
```

> **ajv note:** `@stoplight/spectral-*` pulls a transitive `ajv` whose
> `errorMessage` codegen is broken (garbled `{"str":...}` output, `SyntaxError`).
> This crashes Spectral on **every** Node version tested (18, 20, 22, 24), so it
> is not the "Node 22" issue and is not fixed by pinning Node. `package.json`
> pins `ajv` via an `overrides` block to a working version. With that in place
> Spectral runs cleanly on all of them, so there is **no Node pin** — CI follows
> the repo `.nvmrc` (24), within Spectral's declared support of `node >= 20.17`.

## Guarding the guard

- `.github/CODEOWNERS` protects the ruleset, functions, workflows, and governance
  files — loosening a rule needs steward approval.
- Exemptions are first-class: every waiver in `governance/exemptions.yaml` must
  pass `governance/exemptions.schema.json` (justification ≥30 chars, owner,
  tracking issue) **and** carry an `expires` date. `validate-exemptions.mjs`
  fails the build on any expired waiver and emits `.spectral.effective.yaml`
  (base + waivers), which CI lints — a waiver cannot exist without governance.
- `governance-drift.yml` re-asserts branch protection and re-lints `main` daily.

## Making it unskippable

`governance/branch-protection.json` wires every CI job as a required status
check, sets `enforce_admins: true`, `strict: true`, and
`require_code_owner_reviews: true`. Apply it (admin action, out of band):

```bash
REPO=<org>/thunderid BRANCH=main ./scripts/apply-branch-protection.sh
```

The six required contexts are: `exemptions`, `api-lint`, `codegen-drift`,
`breaking-changes`, `contract-tests`, `coverage-meta`.

## Rolling the gate out to another resource

1. Add `operationId`s (and descriptions/contact) to the target `api/<resource>.yaml`.
2. Lint it: `npx spectral lint ../api/<resource>.yaml -r .spectral.effective.yaml`.
3. For genuine gaps (unimplemented filter/sort/idempotency, problem+json), add a
   time-boxed entry to `governance/exemptions.yaml` with a tracking issue — do
   not downgrade the rule.
4. Add a contract-test package under `tests/integration/contract/` referencing
   every operationId, and point the gate's `SPEC`/`CONTRACT_DIR` at it.
