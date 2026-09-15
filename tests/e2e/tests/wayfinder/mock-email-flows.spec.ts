// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder Mock-Email-Inbox E2E Tests
 *
 * Groups every Wayfinder tryout test that reads from the shared mock SMTP inbox
 * (samples/apps/wayfinder-sample/smtp-server, served at http://localhost:8788): password recovery
 * for the standalone sample app, and the Console-driven staff onboarding invite flow.
 *
 * The inbox has no per-recipient filtering - MockEmailAppPage.openEmailBySubject() matches the
 * newest message with a given subject across the WHOLE inbox, not just the one this test sent. Two
 * of these tests running at once (even in different browsers) could each grab the other's email
 * and use its link, so this file is `.serial` AND runs in its own chromium-only project
 * ("wayfinder-mock-email" in playwright.config.ts) rather than the fully-parallel,
 * three-browser "*-wayfinder-tryout" projects the rest of tests/wayfinder/** runs in.
 *
 * Requires wayfinder-sample-setup.spec.ts to have imported the Wayfinder bundle first (the seed
 * user, the Staff user type, and the Support/DestinationsAdmin roles only exist afterward) -
 * enforced structurally by the wayfinder-setup -> wayfinder-mock-email project dependency in
 * playwright.config.ts, not by any import step in this file.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - SERVER_URL: ThunderID server URL for direct API calls (defaults to https://localhost:8090)
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials
 * - WAYFINDER_APP_URL: URL of the running Wayfinder sample app (e.g. http://localhost:5173) -
 *   only the password-recovery test needs this; it skips itself when unset (matches
 *   b2c-tryout/authentication.spec.ts's WAYFINDER_APP_URL pattern).
 */

import type { APIRequestContext } from "@playwright/test";
import { test, expect, UsersPage, UsersApi, ConsoleRoutes } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";
import { sendOk } from "../../utils/api-request";
import { WayfinderAppPage, MockEmailAppPage } from "../../pages/wayfinder-sample";

const baseUrl = process.env.BASE_URL;
if (!baseUrl) {
  throw new Error("BASE_URL environment variable is required");
}

const wayfinderUrl = process.env.WAYFINDER_APP_URL;

// Seed user shipped with the Wayfinder bundle - see
// samples/apps/wayfinder-sample/thunderid-config/redirect/thunderid.env
const SEED_USERNAME = "john.doe";
const SEED_PASSWORD = "john.doe";

const ONBOARDING_FLOW_HANDLE = "wayfinder-onboarding-flow";

const staffMembers = [
  { role: "Support", user: TestDataFactory.createUser() },
  { role: "DestinationsAdmin", user: TestDataFactory.createUser() },
] as const;

test.describe.serial("Wayfinder Mock Email Inbox", { tag: [TestTags.WAYFINDER] }, () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let originalFlowConfig: any;

  test.beforeAll(async ({ request }) => {
    const response = await sendOk(request, "GET", "/server-config/flow");
    originalFlowConfig = (await response.json()).writable ?? {};

    await sendOk(request, "PUT", "/server-config/flow", {
      ...originalFlowConfig,
      userOnboardingFlow: {
        ...(originalFlowConfig.userOnboardingFlow ?? {}),
        defaultHandle: ONBOARDING_FLOW_HANDLE,
      },
    });
  });

  test.afterAll(async ({ request }) => {
    if (originalFlowConfig !== undefined) {
      await sendOk(request, "PUT", "/server-config/flow", originalFlowConfig);
    }

    const usersApi = new UsersApi(request);
    for (const { user } of staffMembers) {
      const deleted = await usersApi.deleteByUsername(user.username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${user.username}`);
    }
  });

  /** Resolve a role's id by name, needed to read back its assignments. */
  const findRoleId = async (request: APIRequestContext, roleName: string): Promise<string> => {
    const response = await sendOk(request, "GET", "/roles?limit=100");
    const body = (await response.json()) as { roles?: Array<{ id: string; name: string }> };
    const role = body.roles?.find(r => r.name === roleName);
    expect(role, `GET /roles should include "${roleName}"`).toBeDefined();
    return role!.id;
  };

  /** Ids of the users a role is currently assigned to. */
  const findRoleAssignedUserIds = async (request: APIRequestContext, roleId: string): Promise<string[]> => {
    const response = await sendOk(request, "GET", `/roles/${roleId}/assignments?type=user&limit=100`);
    const body = (await response.json()) as { assignments?: Array<{ id: string }> };
    return body.assignments?.map(a => a.id) ?? [];
  };

  test("Reset an existing user's password via the emailed link", async ({ page, context }) => {
    test.skip(!wayfinderUrl, "WAYFINDER_APP_URL not set");

    const wayfinderPage = new WayfinderAppPage(page);

    await wayfinderPage.goto(wayfinderUrl!);
    await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
    await wayfinderPage.clickSignInButton();
    await wayfinderPage.verifyLoginPageLoaded();
    await wayfinderPage.clickResetPasswordLink();
    await wayfinderPage.verifyResetPasswordPageLoaded();
    await wayfinderPage.fillResetPasswordForm(SEED_USERNAME);
    await wayfinderPage.clickSendRecoveryLinkButton();
    await wayfinderPage.verifyResetPasswordConfirmationScreenLoaded();

    // Get the reset link from the email sent to the user using the mock email service.
    const mailPage = await context.newPage();
    const mockEmailPage = new MockEmailAppPage(mailPage);
    await mockEmailPage.goto();

    // Wait until load, click the email, and click the reset link in the email body
    await mockEmailPage.openEmailBySubject(/Reset your password/i);
    const resetPage = await mockEmailPage.clickLinkInEmail(/reset password/i);

    // Now on the reset password form page in the new tab
    const wayfinderResetPage = new WayfinderAppPage(resetPage);
    await wayfinderResetPage.verifyNewPasswordPageLoaded();
    await wayfinderResetPage.fillNewPasswordForm(SEED_PASSWORD);
    await wayfinderResetPage.clickResetPasswordSubmitButton();
    await wayfinderResetPage.verifyPasswordResetSuccessful();

    await resetPage.close();
    await mailPage.close();
  });

  for (const { role, user } of staffMembers) {
    test(`Invite a Staff member and auto-attach the ${role} role on accept`, async ({
      usersPage,
      isolatedPage,
      usersApi,
      request,
    }) => {
      await test.step("Open Add User and select the Staff user type", async () => {
        await usersPage.goto();
        await usersPage.clickAddUser();
        await usersPage.page.waitForURL(`**${ConsoleRoutes.users}/add`, { timeout: Timeouts.PAGE_LOAD });
        await usersPage.selectUserTypeAndContinue("Staff");
      });

      await test.step(`Pick ${role} and send the invitation`, async () => {
        await usersPage.selectStaffRole(role);
        await usersPage.fillInviteeEmail(user.email);
        await usersPage.clickSendInvitation();
        await expect(usersPage.invitationSentHeading).toBeVisible({ timeout: Timeouts.FORM_LOAD });
      });

      await test.step("Open the invite email and complete the invitee's profile", async () => {
        const mockEmailPage = new MockEmailAppPage(isolatedPage);
        await mockEmailPage.goto();
        await mockEmailPage.openEmailBySubject(/you're invited to register/i);
        const acceptPage = await mockEmailPage.clickLinkInEmail(/complete registration/i);

        await new UsersPage(acceptPage, baseUrl).completeRegistrationFlow(user);

        // The registration_complete node renders only TEXT components ("Welcome to Wayfinder" /
        // "Your staff account is ready.") with no submit button - see the flow's
        // registration_complete_heading component in the Wayfinder bundle.
        await expect(acceptPage.getByRole("heading", { name: /welcome to wayfinder/i })).toBeVisible({
          timeout: Timeouts.FORM_LOAD,
        });
        await expect(acceptPage.locator('form button[type="submit"]')).toHaveCount(0, {
          timeout: Timeouts.FORM_LOAD,
        });

        await acceptPage.close();
      });

      await test.step(`Verify the ${role} role was attached automatically`, async () => {
        const createdUser = await usersApi.findByUsername(user.username);
        expect(createdUser, `${user.username} should have been provisioned on accept`).toBeDefined();

        const roleId = await findRoleId(request, role);
        const assignedUserIds = await findRoleAssignedUserIds(request, roleId);
        expect(assignedUserIds, `${role} role's assignments should include ${user.username}`).toContain(
          createdUser!.id
        );
      });
    });
  }
});
