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

## Projects

For a resource that can belong to a project (see `ARCHITECTURE.md`):

- Decide which levels the type allows. A project-level type is meant to be in a project: organization units, themes, layouts, notification templates, and verifiable credentials require one on the control plane, and applications, agents, and flows still accept a missing one while clients migrate (`projectRequired` below). A type that can be at either level (users, groups, roles, resource servers) takes an optional `projectId`. An organization-level type (user types, agent types, connections) has no `PROJECT_ID` column, and the rules below do not apply to it.
- A nullable `PROJECT_ID VARCHAR(36)` column and a `(DEPLOYMENT_ID, PROJECT_ID)` index, in both dialects. A row without a project ID is organization-level.
- `projectId` on create, update, and list in `api/<resource>.yaml`. An update without `projectId` keeps the current project. A list with `projectId` can be narrowed to one organization unit with an `ouId` filter, and a project that does not exist returns an empty page, not an error.
- The declarative and composite stores carry `projectId` and list by project across both stores. The exported document carries `projectId` at its top level, where a project-scoped gateway deploy reads which project the resource belongs to. The importer copies `projectId` into the create request, and the type comes after `resourceTypeProject` in `resourceDependencyOrder` (`internal/system/importer/service.go`).
- The service checks the project through an interface it declares itself: `ValidateProjectOU(ctx, projectID, ouID)` for a resource placed in an organization unit, or `ProjectExists(ctx, id)` for one that is not, such as organization units, themes, and layouts. The control plane wires the project service in (through `Initialize`, or `SetProjectChecker` for organization units) and the data plane wires none, so the data plane stores the project ID without checking that the project exists.
- On every plane, refuse a project ID longer than the 36-character column, as groups, organization units, notification templates, verifiable credentials, themes, and layouts do.
- A package the data plane links never imports `internal/project`; `cmd/server/surface_test.go` fails if it does. A service that calls `ValidateProjectOU` matches its errors by the codes in `internal/system/projectscope/error_codes.go` and returns its own invalid-project and organization-unit-not-in-project errors.
- To require a project on the control plane, add the type to `projectRequired` in `internal/system/projectscope/project_required.go` and to `project_required_test.go`, set to `false` while existing clients still create it without one. Read it with `projectscope.ProjectRequired` only on a deployment that serves projects, and return the resource's own project-required error. Themes and layouts instead refuse a missing project whenever their project checker is set.
- Register a check with `projectService.AddResourceCheck` in `cmd/cpserver/servicemanager.go`, so a project that still holds the resource cannot be deleted. The project service checks entities (users, applications, agents) itself. Roles and resource servers do not register one yet.
- MCP tools that create or list the resource accept `projectId`. Coverage is uneven today: the role and resource server tools, `list_agents`, and `update_agent` take none.
- In `console-cp`, mount the pages in `projectDashboardRoutes()` or `projectFullScreenRoutes()` and add the domain to `PROJECT_SCOPED_DOMAINS` (see `frontend/AGENTS.md`, Project Scope).
- For a new resource, integration tests for both planes: `tests/integration/controlplane/project/project_<resource>_test.go` for the control plane, and `tests/integration/<area>/<resource>_project_test.go` for the data plane, where the project ID is kept unchecked.

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
