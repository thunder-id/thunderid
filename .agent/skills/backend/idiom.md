# Idiom

## Constants and Reuse

- When a value is a recognizable domain concept with a fixed set of values, define a named type for it and typed constants (Ex: `type ChannelType string` with `ChannelTypeEmail`, `ChannelTypeSMS`). Reuse an existing type before adding one. Plain one-off strings do not need a type.
- Header names, content types, and scheme names come from `internal/system/constants`.
- No magic numbers or repeated literals.
- Check `internal/system/utils` before writing a helper. A general helper that is missing goes there.
- Do not wrap an existing util. Call it.
- Do not add a second component for something that exists: an interface with the same methods as an existing one, a second alias for a constant, or a type that only wraps another.
- Use what the Go version in `backend/go.mod` provides (`min`, `max`, `slices`, `maps`, `for range n`, `new(expr)`). Do not hand-roll it.
- Executors that do the same thing differently use an executor mode, not duplicated executors.

## User-Facing Text

User-facing text (notification copy, labels, date formats) lives in templates or i18n resources, never in Go strings, so it can be localized and edited.

## Naming

- Every new exported name says what it is for, read without context.
- Do not give an IAM term (`scope`, `subject`, `claim`, `token`, `session`, `grant`, `assertion`, `resource`, `audience`) a second meaning.
- Do not reuse `Provider`, `Executor`, `Handler`, `Store`, or `Service` for something that is not one.
- Do not name a package `config` or `model` unless it is the package's config or model sub-package. `<pkg>/config` already means the package's configuration.
- Keep names short.
- Keep the product name off surfaces customers' integrators see (URNs, schema and claim names, headers, error messages). Make it configurable or leave it out.

## Comments

- Keep comments minimal. Comment only what the code cannot say.
- A comment states a responsibility or what something does. It does not walk through the logic or processing steps.
- Every new type, function, and method, private ones included, has a short doc comment.
- Test methods generally don't need comments. Add one only to explain what an unclear test checks, or why it handles a case differently.
- Do not explain backend code by what the Console or one caller needs.

## Lint

- Fix lint findings instead of adding `//nolint` or complexity ignores. Accepted exceptions, each with a reason on the directive:
  - `//nolint:lll` on a struct field whose tag forces the length.
  - `//nolint:gosec` on a false positive credential warning (G101) for a constant that only looks like a secret, such as a token field key or an executor name.
- Lint does not catch these, so check them: a shadowed `err`, errors assigned to `_`, single-value type assertions on store rows or `any`, `context.Background()` created inside a request path, and a `switch` missing a new constant.
