// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Accessibility Testing Utilities
 *
 * Production-grade a11y testing powered by axe-core.
 * Provides shared helpers for running WCAG compliance audits
 * across E2E test suite.
 *
 * @see https://github.com/dequelabs/axe-core
 * @see https://www.w3.org/WAI/WCAG22/quickref/
 *
 * @example
 * import { expectNoA11yViolations } from '../../utils/accessibility';
 *
 * test('homepage is accessible', async ({ page }) => {
 *   await page.goto('/');
 *   await expectNoA11yViolations(page);
 * });
 */

import AxeBuilder from "@axe-core/playwright";
import { Page, TestInfo } from "@playwright/test";

// ─── Types ───────────────────────────────────────────────────────────────────

/**
 * Severity levels for axe-core violations.
 * @see https://github.com/dequelabs/axe-core/blob/develop/doc/rule-descriptions.md
 */
export type A11ySeverity = "minor" | "moderate" | "serious" | "critical";

/**
 * Configuration options for accessibility checks.
 */
export interface A11yOptions {
  /** WCAG tag sets to validate against (e.g., 'wcag2a', 'wcag22aa'). Used when runAllRules is false. Defaults to WCAG 2.2 AA. */
  tags?: readonly string[];

  /** Specific axe-core rule IDs to include (runs only these rules). */
  includeRules?: string[];

  /** Specific axe-core rule IDs to exclude from the audit. */
  excludeRules?: string[];

  /** CSS selectors for elements to include in the audit scope. */
  includeSelectors?: string[];

  /** CSS selectors for elements to exclude from the audit scope. */
  excludeSelectors?: string[];

  /**
   * Minimum severity level that causes a test failure.
   * Violations below this threshold are logged as warnings.
   * @default "serious"
   */
  failOnSeverity?: A11ySeverity;

  /**
   * If true, attach a detailed JSON report to the Playwright test results.
   * Useful for HTML report inspection.
   * @default true
   */
  attachReport?: boolean;

  /**
   * If provided, disables the default WCAG tags and runs all enabled rules.
   * Useful when you want to run best-practice checks beyond WCAG.
   * @default false
   */
  runAllRules?: boolean;
}

/**
 * Structured representation of an axe-core violation for reporting.
 */
export interface A11yViolationSummary {
  /** axe-core rule ID (e.g., 'color-contrast', 'label') */
  ruleId: string;

  /** Human-readable description of the violation */
  description: string;

  /** Impact/severity level */
  impact: A11ySeverity;

  /** URL to the axe-core rule documentation */
  helpUrl: string;

  /** Number of DOM nodes affected */
  nodeCount: number;

  /** CSS selectors of affected nodes (first 5) */
  affectedNodes: string[];

  /** WCAG criteria tags (e.g., 'wcag2a', 'wcag412') */
  wcagTags: string[];
}

/**
 * Full a11y audit result returned by `checkA11yWithReport`.
 */
export interface A11yAuditResult {
  /** All violations found */
  violations: A11yViolationSummary[];

  /** Violations that meet or exceed the fail threshold */
  failingViolations: A11yViolationSummary[];

  /** Violations below the fail threshold (warnings) */
  warningViolations: A11yViolationSummary[];

  /** Total number of violated nodes */
  totalViolatedNodes: number;

  /** Whether the audit passes the configured threshold */
  passes: boolean;

  /** Page URL that was audited */
  pageUrl: string;

  /** Timestamp of the audit */
  timestamp: string;
}

// ─── Constants ───────────────────────────────────────────────────────────────

/**
 * Predefined WCAG rule set tag combinations for common compliance targets.
 *
 * @example
 * await expectNoA11yViolations(page, { tags: A11Y_RULE_SETS.WCAG_22_AA });
 */
export const A11Y_RULE_SETS = {
  /** WCAG 2.0 Level A */
  WCAG_20_A: ["wcag2a"],

  /** WCAG 2.0 Level AA (includes Level A) */
  WCAG_20_AA: ["wcag2a", "wcag2aa"],

  /** WCAG 2.1 Level AA (includes 2.0 A + AA) — DEFAULT */
  WCAG_21_AA: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"],

  /** WCAG 2.2 Level AA (includes all prior levels) */
  WCAG_22_AA: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"],

  /** axe-core best practices (beyond WCAG) */
  BEST_PRACTICES: ["best-practice"],

  /** Comprehensive: WCAG 2.2 AA + best practices */
  COMPREHENSIVE: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"],
} as const;

/**
 * Severity levels ordered by ascending impact.
 */
const SEVERITY_ORDER: Record<A11ySeverity, number> = {
  minor: 0,
  moderate: 1,
  serious: 2,
  critical: 3,
};

/**
 * Color-coded indicators for each severity level.
 */
const SEVERITY_COLORS: Record<A11ySeverity, string> = {
  critical: "🔴",
  serious: "🟠",
  moderate: "🟡",
  minor: "🔵",
};

/**
 * Default options for accessibility checks.
 */
const DEFAULT_OPTIONS: Required<A11yOptions> = {
  tags: A11Y_RULE_SETS.WCAG_22_AA,
  includeRules: [],
  excludeRules: [],
  includeSelectors: [],
  excludeSelectors: [],
  failOnSeverity: "serious",
  attachReport: true,
  runAllRules: false,
};

// ─── Core Functions ──────────────────────────────────────────────────────────

/**
 * Creates a configured AxeBuilder instance with the given options.
 * Validates includeSelectors exist on the page before applying them
 * to prevent axe-core "No elements found for include" errors.
 */
async function createAxeBuilder(
  page: Page,
  options: Required<A11yOptions>
): Promise<{ builder: AxeBuilder; skipped: boolean }> {
  let builder = new AxeBuilder({ page });

  // Apply WCAG tag filters (unless runAllRules is true)
  if (!options.runAllRules && options.tags.length > 0) {
    builder = builder.withTags([...options.tags]);
  }

  // Include only specific rules
  if (options.includeRules.length > 0) {
    for (const rule of options.includeRules) {
      builder = builder.withRules(rule);
    }
  }

  // Exclude specific rules
  if (options.excludeRules.length > 0) {
    builder = builder.disableRules(options.excludeRules);
  }

  // Scope to specific elements — validate they exist first
  if (options.includeSelectors.length > 0) {
    const validSelectors: string[] = [];

    for (const selector of options.includeSelectors) {
      const count = await page.locator(selector).count();

      if (count > 0) {
        validSelectors.push(selector);
      } else {
        console.warn(`⚠️ a11y includeSelector "${selector}" matched 0 elements — skipping`);
      }
    }

    if (validSelectors.length === 0) {
      console.log("ℹ️ No includeSelectors matched any elements on the page — skipping audit");

      return { builder, skipped: true };
    }

    for (const selector of validSelectors) {
      builder = builder.include(selector);
    }
  }

  // Exclude specific elements
  if (options.excludeSelectors.length > 0) {
    for (const selector of options.excludeSelectors) {
      builder = builder.exclude(selector);
    }
  }

  return { builder, skipped: false };
}

/**
 * Transforms raw axe-core violation results into structured summaries.
 */
function mapViolations(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  rawViolations: any[]
): A11yViolationSummary[] {
  return rawViolations.map(violation => ({
    ruleId: violation.id,
    description: violation.description,
    impact: violation.impact as A11ySeverity,
    helpUrl: violation.helpUrl,
    nodeCount: violation.nodes?.length ?? 0,
    affectedNodes: (violation.nodes ?? [])
      .slice(0, 5)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      .map((node: any) => node.target?.join(", ") ?? "unknown"),
    wcagTags: (violation.tags ?? []).filter((tag: string) => tag.startsWith("wcag") || tag.startsWith("best-practice")),
  }));
}

/**
 * Partitions violations into failures and warnings based on the severity threshold.
 */
function partitionViolations(
  violations: A11yViolationSummary[],
  failOnSeverity: A11ySeverity
): { failing: A11yViolationSummary[]; warnings: A11yViolationSummary[] } {
  const threshold = SEVERITY_ORDER[failOnSeverity];

  return violations.reduce(
    (acc, violation) => {
      if (SEVERITY_ORDER[violation.impact] >= threshold) {
        acc.failing.push(violation);
      } else {
        acc.warnings.push(violation);
      }

      return acc;
    },
    { failing: [] as A11yViolationSummary[], warnings: [] as A11yViolationSummary[] }
  );
}

// ─── Formatting ──────────────────────────────────────────────────────────────

/**
 * Formats a single violation into a human-readable string.
 *
 * @param violation - Structured violation summary
 * @returns Formatted multi-line string
 */
export function formatViolation(violation: A11yViolationSummary): string {
  const color = SEVERITY_COLORS[violation.impact] ?? "❌";
  const lines = [
    `  ${color} [${violation.impact.toUpperCase()}] ${violation.ruleId}`,
    `     ${violation.description}`,
    `     Affected nodes: ${violation.nodeCount}`,
    `     Selectors: ${violation.affectedNodes.join(" | ") || "N/A"}`,
    `     Help: ${violation.helpUrl}`,
    `     WCAG: ${violation.wcagTags.join(", ") || "N/A"}`,
  ];

  return lines.join("\n");
}

/**
 * Generates a complete report string from a list of violations.
 */
function generateReportString(violations: A11yViolationSummary[], pageUrl: string, label: string): string {
  if (violations.length === 0) {
    return `✅ No ${label} accessibility violations found on: ${pageUrl}`;
  }

  const header =
    `\n🔍 Accessibility Audit Report: ${label}\n` +
    `   Page: ${pageUrl}\n` +
    `   Violations: ${violations.length}\n` +
    `   Total affected nodes: ${violations.reduce((sum, v) => sum + v.nodeCount, 0)}\n` +
    `${"─".repeat(70)}`;

  const body = violations.map(formatViolation).join("\n\n");

  return `${header}\n\n${body}\n\n${"─".repeat(70)}`;
}

// ─── Public API ──────────────────────────────────────────────────────────────

/**
 * Run an accessibility audit and return structured results without asserting.
 *
 * Use this when you need programmatic access to the violations
 * (e.g., for custom reporting or conditional logic).
 *
 * @param page - Playwright Page object
 * @param options - Accessibility check configuration
 * @returns Structured audit result
 *
 * @example
 * const result = await checkA11yWithReport(page);
 * console.log(`Found ${result.violations.length} violations`);
 * if (!result.passes) { /* handle failures *\/ }
 */
export async function checkA11yWithReport(page: Page, options: A11yOptions = {}): Promise<A11yAuditResult> {
  const mergedOptions: Required<A11yOptions> = {
    ...DEFAULT_OPTIONS,
    ...options,
    // If tags were explicitly passed, keep audit tag-scoped unless caller explicitly opts into all rules.
    runAllRules: options.runAllRules ?? (options.tags !== undefined ? false : DEFAULT_OPTIONS.runAllRules),
  };
  const { builder, skipped } = await createAxeBuilder(page, mergedOptions);

  // If all includeSelectors were invalid, return a clean pass
  if (skipped) {
    return {
      violations: [],
      failingViolations: [],
      warningViolations: [],
      totalViolatedNodes: 0,
      passes: true,
      pageUrl: page.url(),
      timestamp: new Date().toISOString(),
    };
  }

  const results = await builder.analyze();
  const violations = mapViolations(results.violations);
  const { failing, warnings } = partitionViolations(violations, mergedOptions.failOnSeverity);

  return {
    violations,
    failingViolations: failing,
    warningViolations: warnings,
    totalViolatedNodes: violations.reduce((sum, v) => sum + v.nodeCount, 0),
    passes: failing.length === 0,
    pageUrl: page.url(),
    timestamp: new Date().toISOString(),
  };
}

/**
 * Assert that the current page has no accessibility violations above the
 * configured severity threshold.
 *
 * This is the **primary shared helper** for accessibility testing.
 * It should be used across all a11y test suites.
 *
 * **Behavior:**
 * - Runs axe-core against the page with the given options
 * - Fails the test if any `critical` or `serious` violations are found (configurable)
 * - Logs `moderate` and `minor` violations as warnings
 * - Optionally attaches a JSON report to Playwright's HTML report
 *
 * @param page - Playwright Page to audit
 * @param options - Configuration for the audit scope and severity
 * @param testInfo - Optional Playwright TestInfo for attaching reports
 * @throws {Error} If violations at or above `failOnSeverity` are found
 *
 * @example
 * // Basic usage — WCAG 2.2 AA, fail on serious+
 * await expectNoA11yViolations(page);
 *
 * @example
 * // Custom WCAG level and scoping
 * await expectNoA11yViolations(page, {
 *   tags: A11Y_RULE_SETS.WCAG_22_AA,
 *   excludeSelectors: ['[data-testid="third-party-widget"]'],
 *   failOnSeverity: 'critical',
 * });
 */
export async function expectNoA11yViolations(
  page: Page,
  options: A11yOptions = {},
  testInfo?: TestInfo
): Promise<void> {
  const result = await checkA11yWithReport(page, options);
  const pageUrl = page.url();

  const runAllRules = options.runAllRules ?? (options.tags !== undefined ? false : DEFAULT_OPTIONS.runAllRules);
  const auditScope = runAllRules ? "all enabled axe-core rules" : (options.tags ?? DEFAULT_OPTIONS.tags).join(", ");

  // Log warnings (below threshold)
  if (result.warningViolations.length > 0) {
    const warningReport = generateReportString(result.warningViolations, pageUrl, "WARNINGS");
    console.warn(warningReport);
  }

  // Attach detailed report to Playwright test results
  if (testInfo && (options.attachReport ?? true)) {
    const reportData = JSON.stringify(
      {
        url: pageUrl,
        timestamp: result.timestamp,
        summary: {
          total: result.violations.length,
          failing: result.failingViolations.length,
          warnings: result.warningViolations.length,
          totalNodes: result.totalViolatedNodes,
        },
        failingViolations: result.failingViolations,
        warningViolations: result.warningViolations,
      },
      null,
      2
    );

    await testInfo.attach("a11y-audit-report", {
      body: reportData,
      contentType: "application/json",
    });
  }

  // Fail on violations above threshold
  if (!result.passes) {
    const failureReport = generateReportString(result.failingViolations, pageUrl, "FAILURES");

    const summary = result.failingViolations.map(v => `${v.impact}: ${v.ruleId} (${v.nodeCount} nodes)`).join(", ");

    throw new Error(`Accessibility violations found on ${pageUrl}:\n` + `${summary}\n\n${failureReport}`);
  }

  // Success
  console.log(`✅ No accessibility violations (${auditScope}) on: ${pageUrl}`);
}

/**
 * Walk the tab sequence and report which elements actually received focus.
 *
 * `expectedFocusableCount` only bounds how many times Tab is pressed and produces a log line when
 * fewer elements are reached; it is not a guarantee, because the tabbable set is browser and OS
 * dependent (on macOS, Firefox and WebKit leave links out of the tab order by default). Assert on the
 * returned `focusedElements`, not on a count derived from a CSS selector.
 *
 * `focusStopped` records that the walk ended because focus stopped advancing. That marks the end of
 * the tab sequence and is not evidence of a keyboard trap: browsers differ on end-of-sequence
 * behaviour, so proving a trap needs a dedicated test of a specific container.
 *
 * @param page - Playwright Page to check
 * @param expectedFocusableCount - How many elements are expected to be reachable, for logging
 */
export async function checkKeyboardNavigation(
  page: Page,
  expectedFocusableCount: number = 1
): Promise<{
  focusedElements: Array<{ tagName: string; role: string | null; ariaLabel: string | null }>;
  focusStopped: boolean;
}> {
  const focusedElements: Array<{ tagName: string; role: string | null; ariaLabel: string | null }> = [];
  const seenSelectors = new Set<string>();
  let previousSelector: string | null = null;
  let focusStopped = false;
  const maxTabs = expectedFocusableCount + 10;

  for (let i = 0; i < maxTabs; i++) {
    await page.keyboard.press("Tab");

    const focusedElement = await page.evaluate(() => {
      const el = document.activeElement;

      if (!el || el === document.body) {
        return null;
      }

      // Identify the element by its position in the DOM tree. A tag name plus a class name is not
      // unique: sibling controls rendered by a component library share generated class names, so
      // every sibling would look like the same element and be misreported as a tab trap.
      const path: string[] = [];
      let node: Element | null = el;
      while (node && node !== document.documentElement) {
        const parent: Element | null = node.parentElement;
        const index = parent ? Array.prototype.indexOf.call(parent.children, node) : 0;
        path.unshift(`${node.tagName.toLowerCase()}:${index}`);
        node = parent;
      }

      return {
        tagName: el.tagName.toLowerCase(),
        role: el.getAttribute("role"),
        ariaLabel: el.getAttribute("aria-label"),
        selector: path.join(">"),
      };
    });

    if (focusedElement) {
      const { selector, ...elementInfo } = focusedElement;

      // Focus stopped advancing, or came back to an element already visited: either way the tab
      // sequence is exhausted, so stop. Neither condition proves a keyboard trap. Browsers disagree
      // about what happens at the end of the sequence - Chromium wraps around to the first element,
      // while on macOS Firefox and WebKit hand focus to the browser chrome, leaving activeElement
      // unchanged - and they also disagree about what is tabbable, since those two leave links out of
      // the tab order by default.
      if (selector === previousSelector || seenSelectors.has(selector)) {
        focusStopped = true;
        break;
      }
      previousSelector = selector;

      seenSelectors.add(selector);
      focusedElements.push(elementInfo);
    }

    // Stop if we've cycled back to the body
    const isBody = await page.evaluate(() => document.activeElement === document.body);

    if (isBody && focusedElements.length > 0) {
      break;
    }
  }

  if (focusedElements.length < expectedFocusableCount) {
    console.warn(
      `⚠️ Expected at least ${expectedFocusableCount} focusable elements, ` + `but only found ${focusedElements.length}`
    );
  } else {
    console.log(`✅ Keyboard navigation: ${focusedElements.length} focusable elements found`);
  }

  return { focusedElements, focusStopped };
}

/**
 * Verify ARIA live regions exist and are properly configured.
 *
 * @param page - Playwright Page to check
 * @returns Array of live region details
 */
export async function checkAriaLiveRegions(page: Page): Promise<Array<{ politeness: string; text: string }>> {
  const liveRegions = await page.locator("[aria-live]").all();
  const results: Array<{ politeness: string; text: string }> = [];

  for (const region of liveRegions) {
    const politeness = (await region.getAttribute("aria-live")) ?? "unknown";
    const text = (await region.textContent()) ?? "";
    results.push({ politeness, text: text.substring(0, 100) });
  }

  console.log(`📢 Found ${results.length} ARIA live region(s)`);

  return results;
}
