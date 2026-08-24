---
paths:
  - "frontend/apps/**/*.tsx"
  - "frontend/apps/**/*.ts"
  - "frontend/packages/**/*.tsx"
  - "frontend/packages/**/*.ts"
---

# Frontend Route Configuration: Never Hardcode a Path

A literal path string is violated in a single keystroke and drifts silently from the route that is actually mounted, so
this is worth checking on every navigation you write.

Every route destination is centralized in a per-app `RouteConfig` (`frontend/apps/console/src/configs/RouteConfig.ts`,
`frontend/apps/gate/src/configs/RouteConfig.ts`) — never a literal path string scattered through app code.

- **Never** write `navigate('/some/path')`, ``navigate(`/some/${id}`)``, or `<Link to="/some/path">`. Always resolve the
  destination through `RouteConfig` (app-local code) or a package's `use<Domain>Routes()` hook (see below).
- Console's `App.tsx` `<Route path>` declarations and `DashboardLayout.tsx`'s sidebar are built from the same
  `RouteConfig`/`ROUTE_SEGMENTS`, so the mounted route and every place that navigates to it share one source and can't
  drift apart. Read `frontend/apps/console/src/configs/RouteConfig.ts` directly — do not trust a copied route table,
  which goes stale.
- A `@thunderid/configure-*` package must never hardcode or assume the host app's URL structure. Each package defines a
  single `src/hooks/use<Domain>Routes.ts` holding all three pieces: the route-shape interface (`<Domain>RoutePaths`), a
  `default<Domain>RoutePaths` constant matching Console's current paths, and the hook itself, built on
  `@thunderid/contexts`'s `useRoutes`. The hook resolves the host-supplied path when present and falls back to the
  package's own default otherwise (see `frontend/packages/configure-users/src/hooks/useUserRoutes.ts`). Components call this hook and build destinations from
  its returned functions — never a literal string. This is what lets a package be mounted under a different URL by a
  different host app without touching the package's code.
- Adding a new route means updating `RouteConfig` (or a package's `use<Domain>Routes.ts`) first, then consuming it — never
  add a `navigate('/new/path')` call without registering the path there.

## Flagging this in review

Treat a `navigate('/literal')`, a template-literal path, or a `<Link to="/literal">` as a defect. Do not flag external
URLs (`http://`, `https://`, `mailto:`), same-page hash fragments, `RouteConfig.ts` itself, or a relative child segment
nested under a parent `<Route>` whose own path already comes from `RouteConfig`/`ROUTE_SEGMENTS` (a plain `"validate"`
child, or a `:paramName` segment). Only the domain's own top-level path must trace back to `RouteConfig`.
