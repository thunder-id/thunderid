# Visibility and Wiring

## Private by Default

- Structs and constructors are private. Expose an instance through `Initialize` in `init.go`.
- What a package hands to another package is an interface, named `<Thing>Interface`.
- A sub-package is not a reason to export a struct or constructor. Give it its own `init.go` with `Initialize` returning an interface, or move the code into the parent.
- A sub-package of free functions still gets a private struct behind a public interface.
- When the parent needs a new instance per call, expose a builder or factory interface. Do not export the constructor.
- Do not export package-level constants that describe a struct. Put them on the struct.
- Do not return an internal slice or map the caller can change.
- Before exporting, check what the consumer actually references. An export only tests use gets unexported.

## `backend/pkg` Is a Library

`backend/pkg/thunderidengine` is imported by embedders as a public library. Anything added there stays there, since changing or removing it breaks them. Add only what an embedder needs, and keep everything else in `backend/internal`.

## Wiring

- Every package is initialized from `Initialize` in `init.go`, called by the service manager with its dependencies. A package never fetches a dependency itself.
- Collaborators (validators, evaluators, renderers) are built once and passed to the service constructor, not created per request.
- A function only the service calls is a receiver method on the service.
- A dependency for a feature that can be off stays optional. `Initialize` may return nil when the feature is disabled, and callers handle nil. A required dependency fails startup for every deployment.

## Background Workers

- A component that starts goroutines exposes a stop method, returned through `Initialize` and called on shutdown. Ex: backchannel logout `dispatcher.Stop`.
- Stopping is bounded by a deadline. It does not wait indefinitely for a queue to drain.
- A job handed to a worker carries only what it needs (Ex: the trace ID and deployment ID), never unnecessary details.

## Config

- A package with extensive config, or config that `thunderidengine` embedders pass in, has a `config` sub-package (package `<pkg>config`) that defines its `Config`. Ex: `oauthconfig`, `flowconfig`.
- `<pkg>config.FromServerRuntime()` transforms the server's deployment config into that `Config`. Only the service manager calls it, and passes the result to `Initialize`.
- Do not pass individual values (a secret, an `enabled bool`) through `Initialize`. Add them to the package config.
- A sub-package uses its parent's config. It does not define its own `Config` or converter.

## Console

The Console is one API client. Backend code and comments do not special-case it.
