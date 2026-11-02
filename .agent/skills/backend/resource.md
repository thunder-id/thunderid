# Management Resources

A resource with a CRUD API and its own store. These rules apply to every new one.

## Fields

- `id` (UUID v7) and an immutable `handle`. References to the resource use the handle.
- Validate the handle with `utils.IsValidHandle`. Reject a changed handle on update.
- The display field is `displayName`.

## Store Modes

- Support `mutable`, `declarative`, and `composite`, chosen by a `<resource>.store` config key that falls back to `declarativeresource.IsDeclarativeModeEnabled()`.
- Files: `store.go`, `cache_backed_store.go`, `file_based_store.go`, `composite_store.go`, `declarative_resource.go`, `config.go`.
- If the PR ships only the DB store, say in the PR that the other modes follow, and link the issue.
- A write to a declared (read-only) resource returns the package's read-only error (`403`), checked before any dependency or in-use check.
- The declarative loader writes straight into the file-based store, with no adapter between them.
- Composite lists use `declarativeresource.CompositeMergeListHelperWithLimit` with raw counts. Do not clamp the counts first.

## Exporter

- Built over the service, not the store.
- `GetAllResourceIDs` pages with `serverconst.MaxPageSize`, skips declared resources, and returns IDs in a stable order (never by ranging a map).
- `ValidateResource` calls `declarativeresource.ValidateResourceName` with a `<RESOURCE>_VALIDATION_ERROR` code.
- Constants `resourceType<X>` and `paramType<X>` in `declarative_resource.go`.
- No test fixtures or test-only loaders in source files.

Register the new type in all of:

- `ExportRequest` in `internal/system/export/model.go`, `resourceMap` in `internal/system/export/service.go`, and `api/export.yaml`
- `importDocument` and `resourceDependencyOrder` in `internal/system/importer/service.go`, keeping the exported `id` on import
- `deleterFor` in `internal/system/importer/delete_runtime.go`
- `configurationExport` in `internal/gateway/version_service.go`

## API Spec

- The handler and its `api/<resource>.yaml` change go in the same PR.
- `DELETE` of a missing resource returns `204`. The spec declares no `404` for it.
- A changed error code, message, or response shape updates every named example that embeds it. A new error does not need a new example.
- The docs build merges all `api/*.yaml` by component name, so a duplicate name silently takes the other file's definition.
  Give a component with resource-specific descriptions or examples, such as a filter parameter, a name unique across `api/`.
  Components that read the same for every resource, such as `limit` and `offset`, can share a name.

## Deployment and Tests

- Add the store key to Helm: the section list in `install/helm/conf/deployment.yaml`, the `stores:` comment in `install/helm/values.yaml`, and the `configuration.stores` row in `install/helm/README.md`.
- Integration tests for declarative and composite modes: a fixture under `tests/integration/resources/declarative_resources/<type>/` and a test like `tests/integration/group/group_declarative_test.go`.
- Export and import coverage under `tests/integration/importexport`.
