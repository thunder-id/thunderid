# API Specs — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md). This directory holds the hand-written OpenAPI 3.0.3 documents for
ThunderID's REST API: 22 specs, one per domain, plus `api/extensions/` for extension-point contracts.

**This file covers authoring. Reviewing these specs belongs to the `docs` skill**, whose `api.md` reference verifies a
spec against the Go backend's actually-registered routes and hard-gates on technical accuracy. Its `SKILL.md` already
routes `api/*.yaml` there, so a request to check, verify, or review a spec should invoke the skill rather than follow
this file. Do not restate its checks here.

## Who Consumes These Files

They are not standalone. `docs/scripts/merge-openapi-specs.mjs` combines them into the rendered reference,
`docs/scripts/generate-postman-collections.mjs` produces Postman collections from them, and
`docs/scripts/cut-version.mjs` snapshots a combined spec per docs version. A spec that does not parse breaks the docs
build, so run `make build_docs` after a structural change.

## Conventions

- **One file per domain**, named for the domain (`user.yaml`, `application.yaml`, `oauth2.yaml`). A new domain gets a new
  file rather than a section appended to an existing one.
- **`openapi: 3.0.3`**, with `info.title`, `info.version`, and an Apache 2.0 `license` block.
- **Templated server variables**, not hardcoded hosts:
  ```yaml
  servers:
    - url: https://{host}:{port}
      variables:
        host: {default: "localhost"}
        port: {default: "8090"}
  ```
- **Declare every `tag` with a description** at the top level, and tag every operation. Tags drive the grouping in the
  rendered reference and in `docs/api-groups.config.yaml`.
- **camelCase property names**, matching the REST API and the Go `json` tags. This is the same rule as
  `backend/AGENTS.md`'s declarative-resource convention.
- **List responses use the standard envelope**: `totalResults`, `startIndex`, `count`, and `links`. Do not invent a
  per-domain pagination shape.
- **Every schema, property, parameter, and response needs a real description.** An empty or placeholder description is
  treated as a defect, because it ships directly into the public reference.
- **Keep the spec and the handler in step.** A new route, a changed request or response shape, or a new query, path, or
  header parameter is not done until the spec matches. The reverse also holds: a public endpoint with no spec entry is a
  gap.

## Validation

- `make build_docs` catches parse errors, unresolvable `$ref`s, and broken merges.
- `make lint_docs` covers the prose side of the docs tree.
- Neither is part of `make pr_checks`, so run them yourself.
