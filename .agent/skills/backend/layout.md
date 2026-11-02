# Layout

## Packages and Files

- Keep packages flat. Add a sub-package only for a separate concern with its own interface.
- Add a `common` sub-package only when a large amount of code is reused across many sub-packages.
- Use the standard file names from the Backend Development overview before inventing new ones.
- The service struct is `<pkg>Service`, built by `new<Pkg>Service`.
- File names are `snake_case` (`event_listener.go`, not `eventlistener.go`), say what the file holds, and follow sibling naming (`declarative_resource.go`, not `declarative.go`).
- Keep related code together. Service logic stays in the service file. Do not split code into extra files without a reason.
- Put a separate concern in its own file. A listener and a hook go in two files.
- A function that works on a struct's state is a receiver method on that struct. Add a package-level function only when it is a genuine standalone helper, since it is callable from anywhere in the package.

## What Goes Where

| Kind | File |
|---|---|
| Error constants and sentinel errors | `error_constants.go` |
| Constants, regexes, limits | `constants.go`, or the existing group of the same kind |
| SQL queries | `store_constants.go` |
| Models and DTOs only | `model.go` |
| Interfaces | the implementing file, or `interface.go` |
| A struct | the file with its receiver methods |
| Helpers, private or shared | `utils.go` |
| Service receiver methods | the service file, never `utils.go` |
| Constants shared across packages | `internal/system/constants` |

`service.go` holds no package-level `var`, `const`, or `type` blocks for errors, regexes, limits, or models.

## Order

- Methods follow the struct definition: exported methods, then private methods, then static private helpers.
- Error codes and query IDs are ascending in file order. A new one takes the next number and goes at the end of its block.

## Package Comment and License

- One `// Package` comment per package, in `service.go`, or in `init.go` when there is no service file. Sibling sub-packages use the same file.
- A new file's license header carries the current year only, never a range.
