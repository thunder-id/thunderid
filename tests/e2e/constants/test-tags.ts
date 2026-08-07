// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Test Tags for organizing and filtering tests
 *
 * Usage in tests:
 * test('my test', { tag: [TestTags.SMOKE, TestTags.USER_MANAGEMENT] }, async ({ page }) => {
 *   // test code
 * });
 *
 * Run specific tags:
 * npx playwright test --grep @smoke
 * npx playwright test --grep-invert @slow
 */

export const TestTags = {
  /** Critical path tests that must pass */
  SMOKE: "@smoke",

  /** Tests that cover happy path scenarios */
  HAPPY_PATH: "@happy-path",

  /** Tests that cover error scenarios */
  ERROR_HANDLING: "@error-handling",

  /** Tests that are known to be slow */
  SLOW: "@slow",

  /** Tests that are flaky and need investigation */
  FLAKY: "@flaky",

  /** User management related tests */
  USER_MANAGEMENT: "@user-management",

  /** Authentication related tests */
  AUTHENTICATION: "@authentication",

  /** Application management tests */
  APPLICATIONS: "@applications",

  /** API related tests */
  API: "@api",

  /** Accessibility (a11y) related tests */
  ACCESSIBILITY: "@accessibility",

  /** Wayfinder sample setup / welcome screen related tests */
  WAYFINDER: "@wayfinder",
} as const;

export type TestTag = (typeof TestTags)[keyof typeof TestTags];
