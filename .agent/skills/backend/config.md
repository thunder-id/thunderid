# Server Configuration Keys

## Files

A new key goes in all of:

- `backend/cmd/server/config/default.json`: the default
- `backend/internal/system/config/config.go`, and `backend/pkg/thunderidengine/config/config.go` for engine config: the field, with snake_case `yaml` and `json` tags
- `install/helm/conf/deployment.yaml`, `install/helm/values.yaml` (camelCase), and the table in `install/helm/README.md`
- the table in `docs/content/deployment/configuration.mdx`

Add it to `backend/cmd/server/deployment.yaml` only when it is a common config that needs a deployment engineer's attention at deploy time.

When removing or renaming a key, find and update every other reference too.

## Naming and Placement

- The key says what it controls, read on its own.
- Do not default to a bootstrap resource id.
- A value tenants change at runtime belongs in the server config API, not deployment config.
- A new server config API section adds its name as a `ConfigName` in `internal/serverconfig/constants.go`.
- How a package receives its config is in `visibility.md`, Config.
