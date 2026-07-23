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

## Build & Test

Use `make` / `pnpm` targets, not Nx (frontend build tooling is migrating to Turborepo).

- **Inner loop**: run the touched feature/page/component tests first (`pnpm test` in the relevant app or package).
- **Final gate**: run only the tests, lints, and Prettier formatting checks targeting the files you changed
  (`pnpm test`, `pnpm lint`, and `pnpm prettier --check` scoped to the affected app or package), not the full frontend
  suite.

## i18n Fallback Values

Every `t('key')` call must pass a fallback default string, either positionally as the second argument (third if
interpolation values follow), e.g. `t('applications:foo.bar', 'Fallback text', {count})`, or as `defaultValue` inside
the options object, e.g. `t('applications:foo.bar', {defaultValue: 'Fallback text', count})`. Both forms are valid;
prefer whichever the surrounding code already uses. This matches the existing convention across the codebase and ensures
the UI degrades gracefully if a key or locale is missing.

## Browser Automation

Load [.agent/skills/console/SKILL.md](../.agent/skills/console/SKILL.md) only when browser navigation or UI verification
is actually required.
