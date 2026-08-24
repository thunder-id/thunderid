---
paths:
  - "tests/e2e/**/*.ts"
  - "tests/e2e/playwright.config.ts"
---

# End-to-End Tests (Playwright)

`tests/e2e/` is its own workspace (`@thunderid/e2e`) driving the real Console and Gate in a browser. It is **excluded
from the root `pnpm lint` and `pnpm test`** by `--filter=!./tests/**`, and `make test_e2e` is **not** part of
`make pr_checks`. Nothing in the default gate will tell you that you broke it, so run it yourself for UI changes.

## Commands

Run from `tests/e2e/`: `pnpm test`, `pnpm test:chromium|firefox|webkit`, `pnpm test:headed`, `pnpm test:debug`,
`pnpm test:ui`, `pnpm report`. Accessibility-tagged tests only: `pnpm test:a11y` (see below). From the repo root,
`make test_e2e` wraps `tests/e2e/run-e2e.sh`, and `pnpm test:e2e` runs the workspace through Turborepo. First run needs
`pnpm test:e2e-prepare` to install browsers. Lint and type-check must be run from `tests/e2e` for the reason above.

## Page Object Model

Tests never hold selectors. Each domain gets a page object under `tests/e2e/pages/<domain>/<name>.page.ts` extending
`BasePage` (`tests/e2e/pages/base.page.ts`), which supplies the shared `page` handle and a `screenshot(name)` debug
helper writing to `tests/e2e/test-results/debug`. Export it through the domain's `index.ts`.

Fixtures compose what a test receives, in `tests/e2e/fixtures/console/`: `console-auth.fixture.ts` (authentication),
`console-pom.fixture.ts` (page objects), `console-routes.fixture.ts` (routes), `console-support.fixture.ts` (support
helpers). Add a new page object to the POM fixture rather than instantiating it inside a test.

Shared helpers live in `tests/e2e/utils/`, organized by concern: `api-request`, `assertions`, `authentication`,
`server-setup`, `test-data`, `visual-testing`, `accessibility`, per-domain API helpers (`users-api`, `applications-api`,
`flows-api`, `connections-api`, `user-types-api`), and mock identity providers (`mock-github-oauth-server.ts`,
`mock-google-oidc-server.ts`, `mock-sms-server.ts`, `mock-server`). Check here before writing a new helper.

## Authentication is per worker, deliberately

Each Playwright worker logs in on first use and reuses its own storage state for the rest of its tests. This is not just
an optimization, and it must not be "improved" into a single shared login: the backend rotates refresh tokens and
revokes the entire token family when a used one is replayed, so workers sharing one session race each other's silent
token refresh and cascade into logouts. If you are tempted to hoist login into `global-setup.ts`, this is why it is not
there.

## Selectors

The codebase ships roughly 1,350 `data-testid` attributes, and that is the intended hook. Prefer `data-testid` and
accessible roles over CSS or text selectors. If a component you need lacks one, add the `data-testid` in the component
rather than reaching for a brittle structural selector.

## Accessibility

There is an accessibility lane using `@axe-core/playwright`, with helpers in `tests/e2e/utils/accessibility` and tests
tagged `@accessibility`. Run it with `pnpm test:a11y` (or `pnpm test:a11y:verbose`). A new Console surface should carry
an `@accessibility`-tagged test.

## Flaky tests

The determinism rules in `.agent/guides/go-testing.md` apply here too, and browser tests violate them more easily. Wait
on a condition, never a fixed duration. Prefer Playwright's auto-waiting assertions over manual polling. Do not assume
ordering between workers, and do not let one test depend on data another test created.
