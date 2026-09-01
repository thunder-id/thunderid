---
paths:
  - "frontend/packages/**/rolldown.config.js"
  - "frontend/packages/**/package.json"
  - "frontend/apps/*/src/App.tsx"
  - "frontend/apps/*/src/lib/monaco-setup.ts"
---

# Frontend Package Build

Two build-time traps that both fail only in production, where dev aliasing and test mocks hide them.

## rolldown External Subpaths

Every package's `rolldown.config.js` builds `external` from `Object.keys(pkg.dependencies)` and
`Object.keys(pkg.peerDependencies)`, which only matches a dependency's bare specifier (e.g. `@thunderid/logger`).
rolldown matches `external` by exact string, so an import of a **subpath** of that dependency (e.g.
`@thunderid/logger/react`, `@hookform/resolvers/zod`) is not covered and gets inlined into this package's own `dist`
output instead of staying external. For a dependency backed by a React context (e.g. `@thunderid/logger`), this silently
creates a second `createContext()` instance in this package's bundle, so the copy's `useContext` never sees the value
the host app's provider supplies — a `use<Thing> must be used within a <Thing>Provider` crash that reproduces in
production only (dev aliases workspace packages to source, masking it) and never in tests.

When scaffolding a new `frontend/packages/*` package, or adding an import of a subpath (`somepkg/subpath`) from an
existing dependency: add that exact subpath as a string literal to the package's `rolldown.config.js` `external` array,
next to the existing `// Peer dep subpaths are not matched by exact string — add them explicitly.` entries (see
`packages/configure-resource-servers/rolldown.config.js`). Do this for every subpath actually imported by the package's
source, not just `@thunderid/logger/react`.

### Flagging this in review

Cross-reference the package's `package.json` `dependencies`/`peerDependencies` against every subpath import actually
used in `src/` (`grep -rn "from '@thunderid/" src`, and the same for other scoped packages). For each dependency
subpath imported, the exact subpath string must appear as its own literal entry in `external`. A missing entry is a
defect: only the bare package name is covered by the dependency-keys spread, so the subpath gets inlined into this
package's `dist`, and for a context-providing dependency that creates a duplicate context instance that breaks at
runtime in production only.

## Monaco Must Be Bundled, Never Loaded From the CDN

`@monaco-editor/react` loads Monaco from the jsDelivr CDN until `loader.config({monaco})` runs, which breaks
air-gapped deployments and any strict Content Security Policy. The host app configures this once in
`src/lib/monaco-setup.ts`, and it must run before the first editor mounts.

- In the app router, every `lazy()` import of a page that renders a Monaco editor must chain through the setup module:
  ```tsx
  const ApplicationEditPage = lazy(() =>
    import('./lib/monaco-setup').then(() => import('./features/applications/pages/ApplicationEditPage')),
  );
  ```
  A plain `lazy(() => import('...Page'))` for a Monaco-rendering page silently falls back to the CDN. Treat it as a
  defect and ask for the `monaco-setup` gate.
- Gate per page rather than importing `monaco-setup` eagerly at app start, so `monaco-editor` stays out of the initial
  bundle.
- Feature components import the editor directly from `@monaco-editor/react` (and editor types from `monaco-editor`).
  Do not add an app-local wrapper module, and do not call `loader.config(...)` or assign `self.MonacoEnvironment`
  anywhere outside the host app's `src/lib/monaco-setup.ts`. Keeping feature code free of host internals is what allows
  it to be extracted into a standalone `@thunderid/configure-*` package later.

Full rationale, worker setup, and how to add Monaco to a new app:
`docs/content/community/contributing/contributing-code/frontend-development/best-practices.mdx`.
