// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Global timeouts for Playwright E2E tests
 */
export const Timeouts = {
  /** Default timeout for UI actions (clicks, fills) */
  DEFAULT_ACTION: 15000,

  /** Timeout for checking element visibility */
  ELEMENT_VISIBILITY: 10000,

  /** Timeout for loading large forms */
  FORM_LOAD: 10000,

  /** Timeout for full page loads */
  PAGE_LOAD: 15000,

  /** Global test timeout */
  GLOBAL_TEST: 60000,

  /** Wait for network idle state */
  NETWORK_IDLE: 10000,

  /** Search debounce wait */
  SEARCH_DEBOUNCE: 500,

  /** Wizard step transition animation wait */
  STEP_TRANSITION: 300,

  /** Auth initialization wait */
  AUTH_INITIALIZATION: 500,

  /** Timeout for login redirects */
  REDIRECT: 20000,

  /**
   * Budget for a suite-level beforeAll/afterAll that provisions server state (mock servers,
   * connections, flows, application rewiring) rather than running a single test.
   */
  SUITE_SETUP: 60 * 1000,

  /**
   * Wait for the Wayfinder AI agent's chat reply: a real LLM call, often followed by an MCP tool
   * round trip, so it runs far longer than a plain UI action.
   */
  LLM_RESPONSE: 45 * 1000,
} as const;
