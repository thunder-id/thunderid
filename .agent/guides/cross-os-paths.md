---
paths:
  - "tools/**/*.ts"
  - "tools/**/*.js"
  - "tools/**/*.mjs"
  - "docs/scripts/**/*.mjs"
  - "**/*.config.ts"
  - "**/*.config.js"
  - "frontend/packages/build-plugins/**"
  - "frontend/packages/create/**"
---

# Cross-OS Path Handling

This governs build tooling and Node scripts anywhere in the repo, not just the frontend: `tools/`,
`docs/scripts/*.mjs`, `frontend/packages/build-plugins`, `frontend/packages/create`, and every `*.config.{ts,js}`.
Windows is a supported development platform and these bugs only ever reproduce there.

Build tooling and Node scripts must work on Windows as well as macOS/Linux. Node's `path` (`join`, `resolve`, `sep`)
produces the OS-native separator, a backslash on Windows, while many build-tool APIs and identifiers (e.g. Vite module
ids/importers, glob patterns) are always POSIX forward-slash. Never compare an OS-native path against one of these with
`startsWith`/`===`, and never hand a backslash path back to a tool that expects POSIX paths. Normalize to forward
slashes first (e.g. `normalizePath` from `vite`, or replacing `\` with `/`) and compare using a literal `'/'` rather
than `sep`. This applies to any string built from `join`/`resolve`/`realpathSync` that will be matched against, or
returned to, such a tool.
