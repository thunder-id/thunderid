// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Dashboard & Main Views — Accessibility Tests
 *
 * Validates WCAG 2.2 AA compliance on authenticated pages:
 * dashboard, navigation, and user management.
 *
 * These tests use the stored authentication state from the setup project,
 * so they run as an authenticated admin user.
 *
 * @see https://www.w3.org/WAI/WCAG22/quickref/
 */

import { test, expect, routes } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import {
  expectNoA11yViolations,
  checkAriaLiveRegions,
  A11Y_RULE_SETS,
} from "../../utils/accessibility";

/**
 * Known accessibility violations in the current app.
 * TODO: Remove these exclusions as the product fixes each issue.
 *
 * target-size (serious, WCAG 2.2 SC 2.5.8): the sidebar navigation links are smaller than the 24x24
 * CSS px minimum, reported on 11 nodes across the dashboard and list pages (a[href$="home"],
 * a[href$="applications"], a[href$="resource-servers"], a[href$="users"], a[href$="agents"], ...).
 * Reproduces in all three browsers. Fixing it means enlarging the sidebar hit areas in the console.
 */
const KNOWN_VIOLATIONS = ["document-title", "html-has-lang", "target-size"];

/**
 * Elements excluded from the audit scope because of a known, tracked defect. Scoping by selector
 * rather than disabling the rule keeps the rule enforced everywhere else on the page.
 *
 * TODO: Remove .MuiAvatar-root once the avatar palette is fixed in the console.
 * color-contrast (serious, WCAG 2 SC 1.4.3): the initials inside MuiAvatar fall below the AA 4.5:1
 * threshold, reported on 2 nodes. Firefox and WebKit flag it; Chromium computes the ratio slightly
 * differently and does not.
 *
 * TODO: Remove .MuiDataGrid-main once the users list grid exposes the required child roles.
 * aria-required-children (critical): the grid declares an ARIA grid role without the row roles that
 * role requires. Surfaces on Firefox, where the grid body is rendered differently than on Chromium.
 */
const KNOWN_VIOLATION_SELECTORS = [".MuiAvatar-root", ".MuiDataGrid-main"];

test.describe("Accessibility — Dashboard & Main Views @accessibility", () => {
  test.describe("Dashboard Home", () => {
    test.beforeEach(async ({ authenticatedPage: page }) => {
      await page.goto(routes.home, { waitUntil: "networkidle" });

      // The home route is lazily loaded and its Suspense fallback is a spinner, which can still be
      // mounted at network idle. Auditing that intermediate DOM makes the results depend on chunk
      // timing, so wait for the page's own greeting heading first.
      await page.getByRole("heading", { level: 1 }).first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    });

    test(
      "TC-A11Y-DASH-001: Dashboard page meets WCAG 2.2 AA standards",
      async ({ authenticatedPage: page }, testInfo) => {
        await test.step("Run axe-core WCAG 2.2 AA audit on dashboard", async () => {
          await expectNoA11yViolations(
            page,
            {
              tags: A11Y_RULE_SETS.WCAG_22_AA,
              excludeRules: KNOWN_VIOLATIONS,
              excludeSelectors: KNOWN_VIOLATION_SELECTORS,
            },
            testInfo,
          );
        });
      },
    );

    test(
      "TC-A11Y-DASH-002: Dashboard has proper landmark regions",
      async ({ authenticatedPage: page }, testInfo) => {
        await test.step("Verify ARIA landmarks and regions", async () => {
          await expectNoA11yViolations(
            page,
            {
              includeRules: [
                "landmark-banner-is-top-level",
                "landmark-contentinfo-is-top-level",
                "landmark-main-is-top-level",
                "landmark-no-duplicate-banner",
                "landmark-no-duplicate-contentinfo",
                "landmark-no-duplicate-main",
                "landmark-one-main",
                "landmark-unique",
                "region",
              ],
            },
            testInfo,
          );
        });
      },
    );

    test(
      "TC-A11Y-DASH-003: Dashboard has valid heading hierarchy",
      async ({ authenticatedPage: page }, testInfo) => {
        await test.step("Check heading structure across dashboard", async () => {
          await expectNoA11yViolations(
            page,
            {
              includeRules: [
                "heading-order",
                "page-has-heading-one",
                "empty-heading",
              ],
            },
            testInfo,
          );
        });
      },
    );

    test(
      "TC-A11Y-DASH-004: Dashboard ARIA live regions are properly configured",
      async ({ authenticatedPage: page }) => {
        await test.step("Check ARIA live regions for dynamic content", async () => {
          const liveRegions = await checkAriaLiveRegions(page);

          // Validate that any live regions found have valid politeness values
          for (const region of liveRegions) {
            expect(["polite", "assertive", "off"]).toContain(region.politeness);
          }
        });
      },
    );
  });

  test.describe("Navigation", () => {
    test.beforeEach(async ({ authenticatedPage: page }) => {
      await page.goto(routes.home, { waitUntil: "networkidle" });
    });

    test(
      "TC-A11Y-DASH-005: Navigation/sidebar is accessible",
      async ({ authenticatedPage: page }, testInfo) => {
        await test.step("Verify navigation accessibility", async () => {
          await expectNoA11yViolations(
            page,
            {
              includeRules: [
                "link-name",
                "link-in-text-block",
                "aria-required-attr",
                "aria-valid-attr",
              ],
              excludeRules: KNOWN_VIOLATIONS,
              excludeSelectors: KNOWN_VIOLATION_SELECTORS,
            },
            testInfo,
          );
        });
      },
    );

    test(
      "TC-A11Y-DASH-006: Navigation links have descriptive accessible names",
      async ({ authenticatedPage: page }, testInfo) => {
        await test.step("Verify link accessibility", async () => {
          await expectNoA11yViolations(
            page,
            {
              includeRules: [
                "link-name",
                "link-in-text-block",
              ],
            },
            testInfo,
          );
        });
      },
    );
  });

  test.describe("User Management Page", () => {
    test.beforeEach(async ({ authenticatedPage: page }) => {
      await page.goto(routes.users, { waitUntil: "networkidle" });

      // The users grid fetches its rows after the page goes network-idle and renders a loading
      // state until they arrive. Auditing that intermediate DOM makes the results depend on
      // request timing, so wait for a real row (the admin user always exists) first.
      await page.locator(".MuiDataGrid-row").first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    });

    test(
      "TC-A11Y-DASH-007: User management page meets WCAG 2.2 AA standards",
      async ({ authenticatedPage: page }, testInfo) => {
        await test.step("Run full WCAG 2.2 AA audit on user management page", async () => {
          await expectNoA11yViolations(
            page,
            {
              tags: A11Y_RULE_SETS.WCAG_22_AA,
              excludeRules: KNOWN_VIOLATIONS,
              excludeSelectors: KNOWN_VIOLATION_SELECTORS,
            },
            testInfo,
          );
        });
      },
    );

    test(
      "TC-A11Y-DASH-008: User management tables are accessible",
      async ({ authenticatedPage: page }, testInfo) => {
        await test.step("Verify table accessibility", async () => {
          await expectNoA11yViolations(
            page,
            {
              includeRules: [
                "table-duplicate-name",
                "td-headers-attr",
                "th-has-data-cells",
                "td-has-header",
                "scope-attr-valid",
              ],
              includeSelectors: ["table", "[role='table']", "[role='grid']"],
            },
            testInfo,
          );
        });
      },
    );
  });

});
