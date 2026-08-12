# Frontend — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md). The frontend is a pnpm monorepo under `frontend/` with two React apps —
`frontend/apps/console` (admin console) and `frontend/apps/gate` (login, registration, recovery) — plus shared and
feature packages under `frontend/packages/`.

For feature-package layout and app boundaries, see the
[Frontend Development overview](../docs/content/community/contributing/contributing-code/frontend-development/overview.mdx).

## Feature Packages

Console management features ship as `@thunderid/configure-*` packages (e.g. `@thunderid/configure-users`) under
`frontend/packages/`. Shared building blocks live in packages such as `@thunderid/components`, `@thunderid/hooks`,
`@thunderid/contexts`, and `@thunderid/i18n`.

Gate consumes ThunderID's published SDK packages — `@thunderid/react` and `@thunderid/react-router` — from the separate
[javascript-sdks](https://github.com/thunder-id/javascript-sdks) repository. Clone that repository only to develop or
debug the SDK itself, or to test against unreleased SDK changes.

### rolldown External Subpaths

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
next to the existing `// Peer dep subpaths are not matched by exact string - add them explicitly.` entries (see
`packages/configure-resource-servers/rolldown.config.js`). Do this for every subpath actually imported by the package's
source, not just `@thunderid/logger/react`.

## Route Configuration — Never Hardcode a Path

Every route destination is centralized in a per-app `RouteConfig` (`frontend/apps/console/src/configs/RouteConfig.ts`,
`frontend/apps/gate/src/configs/RouteConfig.ts`) — never a literal path string scattered through app code.

- **Never** write `navigate('/some/path')`, ``navigate(`/some/${id}`)``, or `<Link to="/some/path">`. Always resolve the
  destination through `RouteConfig` (app-local code) or a package's `use<Domain>Routes()` hook (see below).
- Console's `App.tsx` `<Route path>` declarations and `DashboardLayout.tsx`'s sidebar are built from the same
  `RouteConfig`/`ROUTE_SEGMENTS`, so the mounted route and every place that navigates to it share one source and can't
  drift apart. Read `frontend/apps/console/src/configs/RouteConfig.ts` directly — do not trust a copied route table,
  which goes stale.
- A `@thunderid/configure-*` package must never hardcode or assume the host app's URL structure. Each package defines
  its own `routes/types.ts` (a route-shape interface + defaults matching Console's current paths) and a
  `use<Domain>Routes()` hook built on `@thunderid/contexts`'s `useRoutes`, which resolves the host-supplied path when
  present and falls back to the package's own default otherwise. Components call this hook and build destinations from
  its returned functions — never a literal string. This is what lets a package be mounted under a different URL by a
  different host app without touching the package's code.
- Adding a new route means updating `RouteConfig` (or a package's `routes/types.ts`) first, then consuming it — never
  add a `navigate('/new/path')` call without registering the path there.

## Error Display

Failed writes belong inline, next to the form or action that failed. Failed reads belong wherever the missing data was
going to render. `useToast` from `@thunderid/contexts` is for confirmations, and for failures that have nowhere on the
page to live.

- A mutation failure renders inline wherever the action has a form or dialog — create wizards, edit pages, and confirm
  dialogs. Put an `<Alert severity="error">` beside the submit control, driven off the mutation's own `error` state. The
  user has to read the reason, correct an input, and retry, and a toast dismisses itself before that. Some messages are
  long and interpolated (e.g. `errors.APP-1039`) and do not fit a snackbar at all.
- Do not put the error toast in the mutation hook. A hook cannot know whether its caller has somewhere to show the
  message, and a hook-level `onError` plus a call-site one is the standard way duplicates get introduced. Hooks keep
  their `onSuccess` toast; failures are the caller's decision.
- Mutations fired straight from a row action, toggle, or menu item may toast, because there is no form to attach to and
  no dialog to keep open. Decide by asking where the user's attention already is and whether they need the text to
  persist while they fix something, not by the HTTP verb.
- Mutation successes stay toasts — `showToast(t('create.success'), 'success')` in the hook's `onSuccess`. That is
  already the pattern in every `use{Create,Update,Delete}*` hook.
- Read failures render in place of the data. If a query is a surface's primary content — a list, a grid, a detail page,
  a tab body — render an error component where that content would have gone, with a way to retry. A toast alongside it
  is fine, but never instead of it: a toast auto-dismisses and leaves the user staring at an empty page with no
  explanation. If the query is secondary and the page is still usable without it (a picker's options, an optional count,
  a background prefetch), a toast on its own is right, because there is nowhere natural to put it inline. React Query
  has no query `onError`, so a query hook cannot toast on its own — reaching for one means adding a render-phase or
  effect watcher on `isError`, which is a signal the error belongs inline.
- Render every read failure with `QueryErrorNotice` from `@thunderid/components` rather than a hand-rolled
  `ListingTable.EmptyState` or `<Alert>`. Use `variant="block"` (the default) for a list, grid, or page body, and
  `variant="inline"` for a tab section or in-card region. It takes the error and `t` and resolves the message itself —
  there is no `description` prop, so `error.message` is not representable through it. Pass `resolveErrorMessage` for a
  feature-specific resolver (e.g. `getUserErrorMessage`); it defaults to `getErrorMessage`. Pass `onRetry` for the
  default refresh button and `action` for a second action stacked below it (e.g. an edit page's "Back to X"); with no
  `onRetry`, `action` renders alone.
- Clear an inline error as soon as the user acts on it. Any change to the form invalidates the message, so a
  duplicate-name error must disappear when the name is edited rather than sitting there contradicting the field. Reset
  the mutation (`mutation.reset()`) or the local error state from whatever the form's field-change path is, and on
  cancel or reset too. A stale error next to a now-valid form is worse than no error.
- Never surface server-returned error text. No `error.message`, no `response.data.message`, and no `description`
  `defaultValue` from the error envelope. That text is unlocalized, is written for API consumers rather than end users,
  and leaks backend and HTTP wording into the product. This applies to read failures exactly as much as to writes. Note
  that `error.message ?? t('fallback')` never reaches the fallback — `Error.message` is always a string — so that idiom
  silently guarantees the raw text wins.
- Resolve every message through the error catalog: `getErrorMessage` from `@thunderid/utils`, or a feature-specific
  wrapper such as `getApplicationErrorMessage`, so a backend error code maps to `errors.<CODE>` with a generic fallback
  key. Pass the fallback's default string too, per the i18n Fallback Values rule below, so a missing key degrades to
  readable English instead of rendering `create.error` at the user. Codes shared across services (e.g. `SSE-4030` for an
  authorization denial) resolve from `common:errors.<CODE>` when the feature namespace has no entry, so they need
  mapping only once. This requires `t` to forward an explicit `ns:` prefix, so a per-call-site namespace wrapper must
  pass keys that already carry one straight through.
- Keep one surface per failure. TanStack Query fires both `useMutation({onError})` and the per-call
  `mutate(vars, {onError})`, so defining an error surface in both is how duplicates appear. Two toasts for one failure
  are worse than visibly duplicated: the second silently replaces the first, since `ToastProvider` holds only one
  message at a time, so the user may never see the specific one.
- An error from a dialog belongs in that dialog. If the action was fired from a modal, close the modal only in the
  success path and render the `<Alert>` inside it, next to the confirm button. Closing on submit, or rendering the error
  on the surface behind the modal, leaves the user looking at a re-enabled dialog with no explanation. When the parent
  component owns the mutation because it also serves a non-dialog path, pass the error into the dialog as a prop rather
  than rendering it in the parent. Never discard the user's input — a selection, a filled form — on failure; only clear
  it once the mutation succeeds, or the error costs them the whole task.

## Build & Test

Use `make` / `pnpm` targets, not Nx (frontend build tooling is migrating to Turborepo).

- **Inner loop**: run the touched feature/page/component tests first (`pnpm test` in the relevant app or package).
- **Final gate**: run only the tests, lints, and Prettier formatting checks targeting the files you changed
  (`pnpm test`, `pnpm lint`, and `pnpm prettier --check` scoped to the affected app or package), not the full frontend
  suite.

## Cross-OS Path Handling

Build tooling and Node scripts must work on Windows as well as macOS/Linux. Node's `path` (`join`, `resolve`, `sep`)
produces the OS-native separator, a backslash on Windows, while many build-tool APIs and identifiers (e.g. Vite module
ids/importers, glob patterns) are always POSIX forward-slash. Never compare an OS-native path against one of these with
`startsWith`/`===`, and never hand a backslash path back to a tool that expects POSIX paths. Normalize to forward
slashes first (e.g. `normalizePath` from `vite`, or replacing `\` with `/`) and compare using a literal `'/'` rather
than `sep`. This applies to any string built from `join`/`resolve`/`realpathSync` that will be matched against, or
returned to, such a tool.

## i18n Fallback Values

Every `t('key')` call must pass a fallback default string, either positionally as the second argument (third if
interpolation values follow), e.g. `t('applications:foo.bar', 'Fallback text', {count})`, or as `defaultValue` inside
the options object, e.g. `t('applications:foo.bar', {defaultValue: 'Fallback text', count})`. Both forms are valid;
prefer whichever the surrounding code already uses. This matches the existing convention across the codebase and ensures
the UI degrades gracefully if a key or locale is missing.

## Browser Automation

Load [.agent/skills/console/SKILL.md](../.agent/skills/console/SKILL.md) only when browser navigation or UI verification
is actually required.
