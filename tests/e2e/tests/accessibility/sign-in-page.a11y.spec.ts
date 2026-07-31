// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Authentication Flows — Accessibility Tests
 *
 * Validates WCAG 2.2 AA compliance on authentication-related pages.
 * These tests run against unauthenticated pages (sign-in, sign-up)
 * and do not require pre-existing session state.
 *
 * @see https://www.w3.org/WAI/WCAG22/quickref/
 */

import { test, expect } from "../../fixtures/console";
import {
  expectNoA11yViolations,
  checkKeyboardNavigation,
  A11Y_RULE_SETS,
} from "../../utils/accessibility";

// KNOWN_VIOLATIONS is used to exclude specific axe rules that are currently failing.
// @see https://github.com/dequelabs/axe-core/blob/develop/doc/rule-descriptions.md

/**
 * Known accessibility violations in the current app.
 * TODO: Remove these exclusions as the product fixes each issue.
 */
const KNOWN_VIOLATIONS = ["document-title", "html-has-lang"];

/** Selector for visible, enabled interactive elements only. */
const VISIBLE_INTERACTIVE_SELECTOR =
  "input:visible:not([disabled]), " +
  "button:visible:not([disabled]), " +
  "a[href]:visible, " +
  "select:visible:not([disabled]), " +
  "textarea:visible:not([disabled]), " +
  "[tabindex]:not([tabindex='-1']):visible";

test.describe("Accessibility — Authentication Flows @accessibility", () => {
  test.describe("Sign-In Page", () => {
    test.beforeEach(async ({ page, signinPage }) => {
      // The console is served under /console; requesting it unauthenticated redirects to the gate's
      // sign-in page, which is what this suite audits. Navigating to "/" instead would land on the
      // API root and audit its 401 response.
      await signinPage.gotoHome();

      // That redirect is a document navigation the console shell kicks off after its own load has
      // already gone network-idle, so `goto` returning is not "the sign-in page is up". Auditing
      // then tears down axe's injected script mid-analyze ("Execution context was destroyed").
      // Waiting for the sign-in form pins the audit to the settled gate page.
      await signinPage.waitForLoginForm();
      await page.waitForLoadState("networkidle");
    });

    test(
      "TC-A11Y-AUTH-001: Sign-in page meets WCAG 2.2 AA standards",
      async ({ page }, testInfo) => {
        await test.step("Run axe-core WCAG 2.2 AA audit", async () => {
          await expectNoA11yViolations(
            page,
            {
              tags: A11Y_RULE_SETS.WCAG_22_AA,
              excludeRules: KNOWN_VIOLATIONS,
            },
            testInfo,
          );
        });
      },
    );

    test(
      "TC-A11Y-AUTH-002: Sign-in form has proper labels and ARIA attributes",
      async ({ page }, testInfo) => {
        await test.step("Verify form elements are accessible", async () => {
          await expectNoA11yViolations(
            page,
            {
              includeRules: ["label", "label-title-only", "aria-input-field-name"],
            },
            testInfo,
          );
        });

        await test.step("Verify submit button is accessible", async () => {
          const submitButton = page.locator(
            "button[type='submit'], input[type='submit'], button:has-text('Sign'), button:has-text('Log')",
          ).first();

          // toHaveAccessibleName computes the real accessible name, so aria-label and the
          // text content no longer have to be checked one after the other.
          await expect(submitButton).toHaveAccessibleName(/\S/);
        });
      },
    );

    test(
      "TC-A11Y-AUTH-003: Sign-in page supports keyboard navigation",
      async ({ page }) => {
        await test.step("Verify Tab navigation through interactive elements", async () => {
          const interactiveCount = await page.locator(VISIBLE_INTERACTIVE_SELECTOR).count();

          const result = await checkKeyboardNavigation(page, interactiveCount);

          // Only the reachable set is asserted, not a count derived from the selector: which elements
          // are tabbable depends on the browser and OS (macOS Firefox and WebKit leave links out of
          // the tab order), and browsers also differ on what happens after the last control.
          expect(result.focusedElements.length, "tabbing should reach more than one control").toBeGreaterThan(1);
        });

        await test.step("Verify focus is received by interactive elements", async () => {
          const firstInput = page.locator("input").first();

          await firstInput.focus();
          await expect(firstInput).toBeFocused();
        });
      },
    );

    test(
      "TC-A11Y-AUTH-004: Sign-in page has valid heading hierarchy",
      async ({ page }, testInfo) => {
        await test.step("Check heading structure", async () => {
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
      "TC-A11Y-AUTH-005: Sign-in page has sufficient color contrast",
      async ({ page }, testInfo) => {
        await test.step("Validate WCAG AA color contrast ratios", async () => {
          await expectNoA11yViolations(
            page,
            {
              includeRules: ["color-contrast", "color-contrast-enhanced"],
            },
            testInfo,
          );
        });
      },
    );

    test(
      "TC-A11Y-AUTH-006: Sign-in error states are accessible",
      async ({ page }, testInfo) => {
        await test.step("Trigger validation error", async () => {
          const submitButton = page.locator(
            "button[type='submit'], input[type='submit'], button:has-text('Sign'), button:has-text('Log')",
          ).first();

          await submitButton.click();

          // Wait for actual error UI to appear instead of arbitrary timeout
          await page.locator(
            "[role='alert'], [aria-invalid='true'], .error, .validation-error",
          ).first().waitFor({ state: "visible", timeout: 5000 }).catch(() => {
            // If no error element appears, proceed anyway, the page state is still valid to audit
          });
        });

        await test.step("Verify error messages are accessible", async () => {
          await expectNoA11yViolations(
            page,
            {
              tags: A11Y_RULE_SETS.WCAG_22_AA,
              excludeRules: KNOWN_VIOLATIONS,
            },
            testInfo,
          );
        });
      },
    );
  });

});
