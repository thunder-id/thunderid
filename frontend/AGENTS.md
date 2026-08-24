# Frontend — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md). The frontend is a pnpm monorepo under `frontend/` with two React apps —
`frontend/apps/console` (admin console) and `frontend/apps/gate` (login, registration, recovery) — plus shared and
feature packages under `frontend/packages/`.

For feature-package layout and app boundaries, see the
[Frontend Development overview](../docs/content/community/contributing/contributing-code/frontend-development/overview.mdx),
and its two siblings, which carry rules not repeated here:
[Conventions](../docs/content/community/contributing/contributing-code/frontend-development/conventions.mdx) (file
naming) and
[Best Practices](../docs/content/community/contributing/contributing-code/frontend-development/best-practices.mdx)
(Monaco editor: it must be bundled, never loaded from the jsDelivr CDN).

## Feature Packages

Console management features ship as `@thunderid/configure-*` packages (e.g. `@thunderid/configure-users`) under
`frontend/packages/`. Shared building blocks live in packages such as `@thunderid/components`, `@thunderid/hooks`,
`@thunderid/contexts`, and `@thunderid/i18n`.

Gate consumes ThunderID's published SDK packages — `@thunderid/react` and `@thunderid/react-router` — from the separate
[javascript-sdks](https://github.com/thunder-id/javascript-sdks) repository. Clone that repository only to develop or
debug the SDK itself, or to test against unreleased SDK changes.

## Invariants

Four rules violated in a single keystroke, so they stay here rather than in a guide. Each links to the guide holding the
full reasoning and the edge cases.

- **Never hardcode a route.** No `navigate('/some/path')`, no template-literal path, no `<Link to="/some/path">`. Resolve
  every destination through the app's `RouteConfig` or a package's `use<Domain>Routes()` hook.
  See [.agent/guides/frontend-routing.md](../.agent/guides/frontend-routing.md).
- **Write failures render inline, read failures render in place.** An `<Alert severity="error">` beside the submit
  control for a mutation; `QueryErrorNotice` from `@thunderid/components` where the data would have gone. Toasts are for
  successes, and for failures with nowhere on the page to live. **Never surface server-returned error text** — resolve
  every message through the error catalog. See
  [.agent/guides/frontend-error-display.md](../.agent/guides/frontend-error-display.md).
- **Every `t()` call passes a fallback default string**, either positionally (`t('key', 'Fallback', {count})`) or as
  `defaultValue` in the options object. Both forms are valid; match the surrounding code. Without it, a missing key
  renders the raw key at the user.
- **New files need the licence header.** Lint-enforced here by `@thunderid/eslint-plugin`'s `copyright-header` rule; see
  the root [AGENTS.md](../AGENTS.md).

## Build & Test

Turborepo drives the frontend. Run `turbo`/`pnpm` scripts from the repo root or the workspace you changed.

- **Inner loop**: the touched app or package only. `pnpm test` inside `frontend/apps/console`, or from the root
  `turbo run test --filter=@thunderid/configure-users`. Packages must be built before an app's tests see your change:
  `cd frontend && pnpm build:packages`.
- **Pre-PR gate**: `make pr_checks` is the authoritative gate, as the root [AGENTS.md](../AGENTS.md) says. Scoped checks
  are the inner loop, not a substitute for it. Note what `pr_checks` does and does not cover on the frontend side:
  `make test_frontend` runs the console and gate app tests **only**, so `frontend/packages/*` tests are not in the gate,
  run `cd frontend && pnpm test:packages` yourself. Console or Gate UI changes also want `make test_e2e`, which is
  outside `pr_checks` too.
- Root `pnpm lint` and `pnpm test` are filtered with `--filter=!./tests/**` (and `test` also excludes `./tools/**`), so
  they silently skip the e2e suite. Run that from `tests/e2e`.
- Formatting is repo-wide, not per-package: `pnpm format:check` at the root (Prettier with
  `frontend/prettier.config.js`). Note `bracketSpacing: false`, so imports are written `{Box, Stack}`.

## Guides

| Trigger | Read |
|---|---|
| Surfacing any mutation or query failure | [.agent/guides/frontend-error-display.md](../.agent/guides/frontend-error-display.md) |
| A route, or any `navigate` / `<Link>` / `<Route>` destination | [.agent/guides/frontend-routing.md](../.agent/guides/frontend-routing.md) |
| A form section using react-hook-form and zod | [.agent/guides/frontend-forms.md](../.agent/guides/frontend-forms.md) |
| A `*EditPage.tsx`, or an `edit-*` child section | [.agent/guides/frontend-edit-pages.md](../.agent/guides/frontend-edit-pages.md) |
| A `rolldown.config.js`, a new `packages/*`, a new dependency subpath import, or a page rendering Monaco | [.agent/guides/frontend-package-build.md](../.agent/guides/frontend-package-build.md) |
| Any `.tsx`: components, icons, styling | [.agent/guides/oxygen-ui.md](../.agent/guides/oxygen-ui.md) |
| Build tooling or a script manipulating filesystem paths | [.agent/guides/cross-os-paths.md](../.agent/guides/cross-os-paths.md) |

Before your first edit, read every guide whose trigger matches a file you are about to change, and state in one line
which ones you loaded. If none match, say so.

## Browser Automation

Load the `console` skill only when browser navigation or UI verification is actually required.
