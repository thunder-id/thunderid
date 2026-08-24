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
  per-domain pagination shape. This is the response side; the request parameters a collection GET must accept are in
  Design Rules below. The envelope is SCIM-shaped, which is also why `filter` uses SCIM filter grammar.
- **Every schema, property, parameter, and response needs a real description.** An empty or placeholder description is
  treated as a defect, because it ships directly into the public reference.
- **Keep the spec and the handler in step.** A new route, a changed request or response shape, or a new query, path, or
  header parameter is not done until the spec matches. The reverse also holds: a public endpoint with no spec entry is a
  gap.

## Design Rules

These are the API design rules for this repository, written as rule IDs so they map onto a linter when one lands.
**Nothing enforces them mechanically today**, so they are the authoring contract and the review checklist both.

| Rule | Severity | What it requires |
|---|---|---|
| `collection-get-has-pagination` | error | Collection GETs must accept `limit` **and** one of `cursor` / `offset` |
| `collection-get-has-filter` | error | Collection GETs must accept `filter`, using SCIM 2.0 filter grammar (RFC 7644 §3.4.2) |
| `collection-get-has-sort` | warn | Collection GETs should accept `sort` |
| `operation-has-standard-errors` | error | Every operation declares `400`, `401`, `403`, `500`. Item endpoints add `404`. Writes add `409` |
| `errors-use-standard-error-schema` | error | Error bodies serve `application/json` or `application/problem+json`, and the JSON body references `Error` or `OAuthError`. `text/plain` is allowed on **500 only** |

### Where the existing specs stand

The specs predate these rules and do not all satisfy them. Treat the rules as the target for anything you touch, and do
not assume a neighbouring operation is a good example:

- **Pagination**: `limit` and `offset` appear in 10 specs; `cursor` appears in none. Offset paging is the current norm.
- **Filter**: only 3 specs accept `filter`. Most collection GETs do not, so adding one is usually new surface.
- **Sort**: no spec accepts `sort` today. This is the `warn` rule for that reason.
- **Standard errors**: across 202 operations, `400` is missing from 25%, `401` from 50%, `403` from 65%, and `500` from
  6%. This is the least-followed rule of the five.
- **Error schema**: an `Error` schema is defined in 18 specs and `OAuthError` in `oauth2.yaml`, which matches the rule.
  No spec uses `application/problem+json` yet, though it is permitted. Every current `text/plain` response (in
  `user.yaml` and `group.yaml`) is on a 500, which conforms.

When you add or change an operation, bring it up to these rules rather than matching the surrounding file. When you
notice an existing operation that violates one and it is outside your change, leave it and say so rather than expanding
the diff.

## Validation

- `make build_docs` catches parse errors, unresolvable `$ref`s, and broken merges.
- `make lint_docs` covers the prose side of the docs tree.
- Neither is part of `make pr_checks`, so run them yourself.
