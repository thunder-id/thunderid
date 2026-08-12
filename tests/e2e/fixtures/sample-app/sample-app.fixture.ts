// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Sample App Fixture
 *
 * Provides page object instances for sample app testing.
 */

import { test as base } from "@playwright/test";
import { SampleAppLoginPage } from "../../pages/sample-app";

type SampleAppFixtures = {
  sampleAppLoginPage: SampleAppLoginPage;
  /**
   * Client id the sample app should boot as (see constants/sample-apps.ts), overriding the real
   * one baked into its public/runtime.json (samples/apps/react-sdk-sample/src/config.tsx fetches
   * that file at module load). An option fixture, not a value fixture: unset by default so specs
   * that don't need isolation (e.g. sample-app-login.spec.ts) get the real sample app untouched.
   * Set per-suite with `test.use({ sampleAppClientId: SampleAppClientIds.MFA })`.
   */
  sampleAppClientId?: string;
};

/**
 * Extended test fixture with sample app page objects.
 */
export const test = base.extend<SampleAppFixtures>({
  sampleAppClientId: [undefined, { option: true }],

  // Overrides the built-in `context` fixture (rather than `page`) so the route is registered
  // before the first navigation and covers every page opened in this context - the social login
  // flow bounces sample-app -> gate -> sample-app, and a route on just `page` would miss any
  // popup or secondary page along the way.
  context: async ({ context, sampleAppClientId }, use) => {
    if (sampleAppClientId) {
      await context.route("**/runtime.json", async route => {
        // Fetch the real file and override only clientId, so a shape change to runtime.json
        // (a new field config.tsx starts reading) can't silently go stale here.
        const response = await route.fetch();
        const body = await response.json();
        await route.fulfill({ response, json: { ...body, clientId: sampleAppClientId } });
      });
    }
    await use(context);
  },

  /**
   * Sample App Login Page fixture
   */
  sampleAppLoginPage: async ({ page }, use) => {
    const sampleAppLoginPage = new SampleAppLoginPage(page);
    await use(sampleAppLoginPage);
  },
});

export { expect } from "@playwright/test";
