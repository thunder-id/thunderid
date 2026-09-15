// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Branded Connection CRUD E2E Tests
 *
 * Tests create/read/update/delete of each branded social connection (identity provider) through
 * the console admin UI. Google and GitHub are both "branded" vendors (see
 * frontend/packages/configure-connections/src/config/connectionVendorMeta.tsx): each is configured
 * via a single-step wizard at /connections/<vendor>/configure, its name is fixed (not
 * user-editable), and its redirect URI is server-derived and read-only. They share one field set
 * (`oauthFields` in connectionFormFields.ts), so they share one suite here, parameterized by vendor.
 *
 * Test Cases (per vendor):
 * - TC001: Create a connection
 * - TC002: Read the created connection's details (secret stays masked)
 * - TC003: Update the connection's scopes
 * - TC004: Delete the connection
 * - TC005: Create is blocked when required fields are missing
 *
 * Required environment variables:
 * - BASE_URL / SERVER_URL: Console/server base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: Admin credentials
 */

import { test, expect } from "../../fixtures/console";
import { ConnectionsApi } from "../../utils/connections-api";
import { TestDataFactory } from "../../utils/test-data";
import type { APIRequestContext } from "@playwright/test";

const VENDORS = [
  { type: "google", displayName: "Google", updatedScopes: "openid email" },
  { type: "github", displayName: "GitHub", updatedScopes: "read:user user:email" },
];

/**
 * Delete this suite's own connection, if a crashed previous run left it behind, so it can't
 * collide with TC001. Scoped to the fixed name the console wizard itself creates (`name:
 * meta.displayName` in ConnectionConfigureWizardPage.tsx, which the backend also enforces as
 * unique) rather than every connection of the vendor's type - other tooling (e.g. the social
 * login E2E suite's own API-created connection) can share that type under a different name and
 * must not be swept up here.
 */
async function deleteExistingConnection(
  request: APIRequestContext,
  vendor: string,
  displayName: string
): Promise<void> {
  const connectionsApi = new ConnectionsApi(request);
  const existing = await connectionsApi.findByName(vendor, displayName);
  if (existing) {
    await connectionsApi.deleteById(vendor, existing.id);
    console.log(`✓ Removed leftover ${vendor} connection: ${existing.id}`);
  }
}

for (const vendor of VENDORS) {
  // Tests share a single created connection's id across the CRUD lifecycle, so they must run
  // in declaration order rather than Playwright's default (parallelizable-within-file) order.
  test.describe.serial(`${vendor.displayName} Connection - CRUD Operations`, () => {
    let connectionId: string | null = null;
    const clientId = TestDataFactory.generateUniqueId(`${vendor.type}-client`);
    const clientSecret = `e2e-${vendor.type}-client-secret`;

    test.beforeAll(async ({ request }) => {
      // Deliberately not tolerant: the wizard hardcodes the connection name, so a leftover the
      // sweep failed to remove makes TC001 fail on a duplicate-name conflict instead.
      await deleteExistingConnection(request, vendor.type, vendor.displayName);
    });

    test.afterAll(async ({ request }) => {
      // Safety net: if a test failed before TC004 could delete it, clean up here. Runs
      // unconditionally (not gated on `connectionId`) since the connection can be created
      // server-side even when getConnectionIdFromUrl() throws before assigning it. Tolerant of
      // failures, unlike beforeAll: a flaky teardown must not mask the verdict a test reported.
      try {
        await deleteExistingConnection(request, vendor.type, vendor.displayName);
      } catch (error) {
        console.warn(`Cleanup skipped for ${vendor.type} connection "${vendor.displayName}": ${String(error)}`);
      }
    });

    test(`TC001: Create a ${vendor.displayName} connection`, async ({ connectionsPage }) => {
      await test.step(`Navigate to the ${vendor.displayName} configure wizard`, async () => {
        await connectionsPage.gotoConfigure(vendor.type);
      });

      await test.step("Fill in client credentials", async () => {
        await connectionsPage.fillOAuthForm({ clientId, clientSecret });
      });

      await test.step("Submit and land on the connection's detail page", async () => {
        await connectionsPage.submitCreate();
        // getConnectionIdFromUrl() throws if the id isn't present, so a truthiness check here
        // would be dead code.
        connectionId = connectionsPage.getConnectionIdFromUrl();
      });

      await test.step("Verify the new connection appears in the list", async () => {
        await connectionsPage.goto();
        await expect(connectionsPage.cardById(vendor.type, connectionId!)).toBeVisible();
      });
    });

    test("TC002: Read the created connection's details", async ({ connectionsPage }) => {
      expect(connectionId, "TC001 must have created a connection").toBeTruthy();

      await connectionsPage.gotoDetails(vendor.type, connectionId!);

      await expect(connectionsPage.clientIdInput).toHaveValue(clientId);
      await expect(connectionsPage.redirectUriField).not.toHaveValue("");
      // The stored secret is never re-sent to the client - it renders as a masked, disabled field.
      await expect(connectionsPage.clientSecretInput).toBeDisabled();
      await expect(connectionsPage.clientSecretInput).not.toHaveValue(clientSecret);
    });

    test("TC003: Update the connection's scopes", async ({ connectionsPage }) => {
      expect(connectionId, "TC001 must have created a connection").toBeTruthy();

      await connectionsPage.gotoDetails(vendor.type, connectionId!);
      await connectionsPage.updateScopes(vendor.updatedScopes);

      // Reload and verify the change persisted server-side, not just in local component state.
      await connectionsPage.gotoDetails(vendor.type, connectionId!);
      await expect(connectionsPage.scopesInput).toHaveValue(vendor.updatedScopes);
    });

    test("TC004: Delete the connection", async ({ connectionsPage }) => {
      expect(connectionId, "TC001 must have created a connection").toBeTruthy();

      await connectionsPage.gotoDetails(vendor.type, connectionId!);
      await connectionsPage.clickTab("Advanced");
      await connectionsPage.delete();

      await expect(connectionsPage.cardById(vendor.type, connectionId!)).toHaveCount(0);
      connectionId = null;
    });

    test("TC005: Create is blocked when required fields are missing", async ({ connectionsPage }) => {
      await connectionsPage.gotoConfigure(vendor.type);

      // No clientId/clientSecret filled in - the create button must stay disabled.
      await expect(connectionsPage.wizardCreateButton).toBeDisabled();

      await connectionsPage.clientIdInput.fill(TestDataFactory.generateUniqueId(`${vendor.type}-client`));
      // clientSecret still empty - still required.
      await expect(connectionsPage.wizardCreateButton).toBeDisabled();
    });
  });
}
