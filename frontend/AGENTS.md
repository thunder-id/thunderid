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

## Console Route Source of Truth

Routes are defined in `frontend/apps/console/src/App.tsx`; sidebar navigation and categories in
`frontend/apps/console/src/layouts/DashboardLayout.tsx`. Read those files directly — do not trust copied route tables,
which go stale.

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
