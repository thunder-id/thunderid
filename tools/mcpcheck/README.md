# mcpcheck

`mcpcheck` is the CI gate that makes every new API operation and every feature specification carry an explicit MCP exposure decision: does an AI agent get a ThunderID MCP tool for this, and if not, why not. It runs in the `MCP Exposure Check` workflow (`.github/workflows/mcp-exposure-check.yml`) on pull requests that touch `api/`, a `docs-internals/<feature>/spec.md`, a backend `tools.go` or `tool.go`, or this tool.

## API operations

Every operation in `api/*.yaml` carries an `x-thunderid-mcp` extension in exactly one of three forms.

The operation is exposed. Each tool is written without its `thunderid_` prefix and is registered in the backend as `thunderid_<name>`. The tier is `read`, `write` or `destructive`.

```yaml
x-thunderid-mcp:
  expose: true
  tools:
    - {name: list_connections, tier: read}
```

The operation is deliberately not exposed, with the reason.

```yaml
x-thunderid-mcp:
  expose: false
  reason: Protocol endpoint; agents call it as OAuth clients, not as a tool.
```

The decision is deferred, with the reason. The tracking issue belongs in the specification, not here.

```yaml
x-thunderid-mcp:
  expose: deferred
  reason: Planned for a later phase.
```

An `x-thunderid-mcp` at the top level of an OpenAPI document decides every operation in that file that has no decision of its own. It may set `expose: false` or `expose: deferred`, never `expose: true`: an exposed operation names its tools on the operation itself.

The check fails when:

- an operation has no decision;
- `expose` is not `true`, `false` or `deferred`;
- `expose: true` has no tools, names a tool that is not registered, or gives a tier other than `read`, `write` or `destructive`;
- `x-thunderid-mcp` or a tools entry carries a key other than `expose`, `tools` and `reason`, or `name` and `tier` (an `issue` key included);
- `expose: true` carries a `reason`;
- `expose: false` or `expose: deferred` has no `reason`;
- `expose: false` or `expose: deferred` lists tools;
- a registered tool is claimed by no operation and is not listed in [`standalone-tools.yaml`](standalone-tools.yaml).

Operations and tools are many to many: one operation can back several purpose-built tools, and one tool can combine several operations, so an operation may list several tools and a tool may be claimed by several operations, here and as several rows of the specification table.

Registered tools are read from the `Name` of every `mcp.Tool` literal in the `tools.go` and `tool.go` files under `backend/internal`. A `Name` that is not a string literal is itself a problem, because the check cannot see the tool. A tool that no API operation backs, such as an SDK integration guide, goes in `standalone-tools.yaml` with a one-line reason.

## Specifications

Every `docs-internals/<feature>/spec.md` a pull request adds or modifies has a `### MCP exposure` section, placed after `### API` when the specification has one. The section holds a `Decision:` line with one of three values.

- `Decision: not required`, with a `Reason:` line.
- `Decision: deferred`, with an `Issue:` line holding `#<n>` or `https://github.com/thunder-id/thunderid/issues/<n>`.
- `Decision: required`, with a table of at least one row.

```markdown
### MCP exposure

Decision: required

| Operation | Tool | Tier |
|---|---|---|
| GET /connections | thunderid_list_connections | read |
| POST /connections/{id}/test | not exposed: runs a live request against the endpoint | |
| DELETE /connections/{id} | deferred #1234 | destructive |
```

The operation is `METHOD /path`. The tool is a tool name (`thunderid_<snake_case>`), `not exposed: <reason>`, or `deferred <issue>`; a bare `not exposed` fails. The tier is `read`, `write` or `destructive`, and a `not exposed` row may leave it empty. The template in `docs-internals/templates/spec.md` carries the section.

## Existing gaps: the ratchet

Given the base branch's directories through `-base-api` and `-base-tools`, the API check runs on both trees and fails only on problems the base does not already have. Problems are compared on the file name, method, path and rule, never on line numbers, so moving an operation within a file does not make it new. The check prints how many problems on the base branch it did not count. Operations that predate the gate therefore block nobody, while a new operation without a decision fails.

The specification check has no ratchet: it runs only on specifications the pull request adds or modifies, so editing an older specification means adding its section.

## Running locally

From the repository root:

```bash
go -C tools/mcpcheck test ./...

go -C tools/mcpcheck run . spec "$PWD/docs-internals/<feature>/spec.md"

go -C tools/mcpcheck run . api -api "$PWD/api" -tools "$PWD/backend/internal" \
  -standalone "$PWD/tools/mcpcheck/standalone-tools.yaml"
```

To compare against the base branch as CI does, check it out as a worktree and pass its directories:

```bash
git fetch upstream main
git worktree add --detach /tmp/mcpcheck-base upstream/main
go -C tools/mcpcheck run . api -api "$PWD/api" -tools "$PWD/backend/internal" \
  -standalone "$PWD/tools/mcpcheck/standalone-tools.yaml" \
  -base-api /tmp/mcpcheck-base/api -base-tools /tmp/mcpcheck-base/backend/internal
git worktree remove /tmp/mcpcheck-base
```

Paths are absolute because `go -C` runs from `tools/mcpcheck`. Each command prints one line per problem, naming the file, the operation or tool, and what to add, and exits 1 when there is any.
