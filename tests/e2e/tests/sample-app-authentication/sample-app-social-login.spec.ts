/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Sample App Social Login Tests
 *
 * Tests the "Continue with <vendor>" login flow against each vendor's mock server
 * (utils/mock-google-oidc-server.ts, utils/mock-github-oauth-server.ts), so no real vendor
 * account or network access is needed. The backend is redirected to the mock via
 * identity_provider.<vendor>_base_url (see backend/internal/idp/utils.go), configured for local
 * runs by run-e2e.sh and for CI by the E2E job in .github/workflows/pr-builder.yml.
 *
 * Test Cases (per vendor):
 * - TC001: Complete login flow, then logout (GitHub only: also verifies a private primary email
 *   is resolved via the user emails endpoint)
 * - TC002: Login does not complete when the vendor denies access
 *
 * Prerequisites (automatically handled):
 * - Sample app running at SAMPLE_APP_URL
 * - The server running at SERVER_URL, with identity_provider.<vendor>_base_url pointed at the
 *   mock's base URL
 * - Mock server for the vendor (automatically started)
 * - Vendor authentication flow (automatically created)
 *
 * Required environment variables:
 * - SAMPLE_APP_URL: URL of the sample app (e.g., https://localhost:3000)
 * - SERVER_URL: URL of the server (default: https://localhost:8090)
 * - MOCK_GOOGLE_BASE_URL / MOCK_GITHUB_BASE_URL: Base URL each vendor's mock server listens on;
 *   a vendor's suite is skipped when its variable is not set
 * - ADMIN_USERNAME / ADMIN_PASSWORD: Admin credentials (default: admin/admin)
 */

import { test, expect } from "../../fixtures/sample-app";
import { SampleAppLoginPage } from "../../pages/sample-app/sample-app-login.page";
import { MockGoogleOIDCServer } from "../../utils/mock-google-oidc-server";
import { MockGitHubOAuthServer } from "../../utils/mock-github-oauth-server";
import { SocialLoginSetup, SocialLoginSetupResult } from "../../utils/server-setup";
import { serverUrl } from "../../utils/api-request";
import { Timeouts } from "../../constants/timeouts";
import { SampleAppClientIds } from "../../constants/sample-apps";

const sampleAppUrl = process.env.SAMPLE_APP_URL;

// The vendor IDP must redirect back to the gate app's own callback route (not the sample app's
// URL): the flow's executionId is resumed from sessionStorage on the gate's origin, which a
// cross-origin landing on the sample app can't read. See getGateCallbackUrl() in
// frontend/packages/contexts/src/Config/ConfigProvider.tsx, which the console's connection
// wizard uses to prefill this same field (read-only there, since it's always this fixed path).
const gateCallbackUrl = `${serverUrl.replace(/\/+$/, "")}/gate/callback`;

const mockGoogleUser = {
  sub: "e2e-google-user-id",
  email: "e2e-google-user@example.com",
  emailVerified: true,
  name: "E2E Google User",
  givenName: "E2E",
  familyName: "Google User",
};

// Email is null on the /user response, as GitHub returns it for an account that keeps its address
// private: the backend then resolves the primary address from /user/emails (TC004).
const mockGitHubUser = {
  id: 987654,
  login: "e2e-github-user",
  name: "E2E GitHub User",
  email: null,
};
const mockGitHubUserEmail = "e2e-github-user@example.com";
// GitHub has no `sub` claim; the backend derives it from the numeric account id, as a string
// (see backend/internal/authn/oauth/utils.go::ProcessSubClaim).
const mockGitHubUserSub = String(mockGitHubUser.id);

const VENDORS = [
  {
    vendor: "google" as const,
    displayName: "Google",
    baseUrlEnvVar: "MOCK_GOOGLE_BASE_URL",
    defaultPort: 8093,
    mockClientId: "e2e-mock-google-client-id",
    mockClientSecret: "e2e-mock-google-client-secret",
    linkedUser: { username: "e2e-google-login-user", email: mockGoogleUser.email, sub: mockGoogleUser.sub },
    createMock(port: number, clientId: string, clientSecret: string) {
      const mock = new MockGoogleOIDCServer(port, clientId, clientSecret);
      mock.addUser(mockGoogleUser);
      return mock;
    },
  },
  {
    vendor: "github" as const,
    displayName: "GitHub",
    baseUrlEnvVar: "MOCK_GITHUB_BASE_URL",
    defaultPort: 8092,
    mockClientId: "e2e-mock-github-client-id",
    mockClientSecret: "e2e-mock-github-client-secret",
    linkedUser: { username: "e2e-github-login-user", email: mockGitHubUserEmail, sub: mockGitHubUserSub },
    createMock(port: number, clientId: string, clientSecret: string) {
      const mock = new MockGitHubOAuthServer(port, clientId, clientSecret);
      mock.addUser(mockGitHubUser, [{ email: mockGitHubUserEmail, primary: true, verified: true }]);
      return mock;
    },
  },
];

for (const v of VENDORS) {
  // Raw (no fallback) so the skip check below reflects whether the backend is actually wired up
  // (identity_provider.<vendor>_base_url), not just whether this constant has a usable value.
  const mockBaseUrlRaw = process.env[v.baseUrlEnvVar];
  const mockBaseUrl = mockBaseUrlRaw || `http://localhost:${v.defaultPort}`;
  const mockPort = Number(new URL(mockBaseUrl).port || String(v.defaultPort));

  const describeOrSkip = sampleAppUrl && mockBaseUrlRaw ? test.describe : test.describe.skip;

  describeOrSkip(`Sample App - ${v.displayName} Social Login`, () => {
    // Both vendors drive the sample app as their own dedicated application
    // (constants/sample-apps.ts), rather than the shared REACT_SDK_SAMPLE default-login uses.
    test.use({ sampleAppClientId: SampleAppClientIds.SOCIAL });

    let mockServer: MockGoogleOIDCServer | MockGitHubOAuthServer;
    let setupResult: SocialLoginSetupResult | null = null;

    test.beforeAll(async ({ request }) => {
      test.setTimeout(Timeouts.SUITE_SETUP);

      console.log(`\n=== ${v.displayName} Social Login Test Suite Setup ===`);

      mockServer = v.createMock(mockPort, v.mockClientId, v.mockClientSecret);
      await mockServer.start();
      console.log(`✓ Mock ${v.displayName} Server started at ${mockServer.getURL()}`);

      const setup = new SocialLoginSetup(request, {
        vendor: v.vendor,
        appClientId: SampleAppClientIds.SOCIAL,
        clientId: v.mockClientId,
        clientSecret: v.mockClientSecret,
        redirectUri: gateCallbackUrl,
        linkedUser: v.linkedUser,
      });

      try {
        setupResult = await setup.setup();
        console.log("✓ Automated setup completed successfully");
      } catch (error) {
        console.error("✗ Automated setup failed:", error);
        throw error;
      }

      console.log("=============================================\n");
    });

    test.afterAll(async ({ request }) => {
      test.setTimeout(Timeouts.SUITE_SETUP);
      console.log(`\n=== ${v.displayName} Social Login Test Suite Teardown ===`);

      if (setupResult) {
        await SocialLoginSetup.cleanup(request, setupResult.cleanupFunctions);
      }

      if (mockServer) {
        await mockServer.stop();
        console.log(`✓ Mock ${v.displayName} Server stopped`);
      }

      console.log("================================================\n");
    });

    async function loginViaProvider(sampleAppLoginPage: SampleAppLoginPage, displayName: string) {
      const completionResponse = await sampleAppLoginPage.loginWithProvider(displayName, sampleAppUrl!);
      const body = await completionResponse.json();

      expect(body.flowStatus, `the flow must report COMPLETE once ${displayName} login finishes`).toBe("COMPLETE");
      expect(body.assertion, "a completed flow must carry an assertion").toBeTruthy();

      await sampleAppLoginPage.verifyLoggedIn();
    }

    // GitHub gets its own test body (rather than a runtime branch inside one shared test) so it
    // can fold in the private-email-resolution check without any conditional inside a test().
    if (v.vendor === "github") {
      test(`TC001: Complete ${v.displayName} login flow, then logout`, async ({ sampleAppLoginPage }) => {
        const mockGitHubServer = mockServer as MockGitHubOAuthServer;
        // Unlike Google, GitHub's user profile carries no email for a private address, so the
        // backend makes a second call to /user/emails for it. Counting that call is what
        // distinguishes GitHub's flow from Google's: the login itself looks identical from the
        // browser's side.
        const emailRequestsBefore = mockGitHubServer.getUserEmailRequestCount();

        await test.step(`Log in via ${v.displayName}`, () => loginViaProvider(sampleAppLoginPage, v.displayName));

        await test.step("Verify a private primary email was resolved via the user emails endpoint", () => {
          expect(
            mockGitHubServer.getUserEmailRequestCount(),
            "the backend must call /user/emails when the profile carries no email"
          ).toBeGreaterThan(emailRequestsBefore);
        });

        await test.step("Log out", async () => {
          await sampleAppLoginPage.logout();
          await sampleAppLoginPage.verifyLoggedOut();
        });
      });
    } else {
      test(`TC001: Complete ${v.displayName} login flow, then logout`, async ({ sampleAppLoginPage }) => {
        await test.step(`Log in via ${v.displayName}`, () => loginViaProvider(sampleAppLoginPage, v.displayName));

        await test.step("Log out", async () => {
          await sampleAppLoginPage.logout();
          await sampleAppLoginPage.verifyLoggedOut();
        });
      });
    }

    test(`TC002: Login does not complete when ${v.displayName} denies access`, async ({ sampleAppLoginPage }) => {
      mockServer.setAuthorizeError("access_denied");

      await sampleAppLoginPage.gotoLoginPage(sampleAppUrl!);
      await sampleAppLoginPage.clickContinueWith(v.displayName);

      // The mock redirects straight back with an OAuth error and no code, so the flow never
      // completes - the gate surfaces an explicit sign-in-failed error instead of logging in.
      await sampleAppLoginPage.verifySignInError();
    });
  });
}
