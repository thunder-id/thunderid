// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder Sample Setup E2E Tests
 *
 * Covers the "Setup Wayfinder Sample" card embedded in the welcome tryout pages: importing the
 * Wayfinder declarative config bundle into ThunderID and verifying every resource it creates
 * (applications, agents, users, user types, resource servers, roles, flows) loaded correctly.
 *
 * Import is idempotent (`upsert:true`), so this suite never deletes what it creates: re-running it
 * against a server that already has the Wayfinder sample configured simply re-upserts the same
 * resources. Tests in this file must run in order (TC004/TC005 assert on data TC003 creates); the
 * describe block is `.serial` to make and enforce that dependency.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - SERVER_URL: ThunderID server URL for direct API calls (defaults to https://localhost:8090)
 * - ADMIN_USERNAME: Admin credentials for authentication
 * - ADMIN_PASSWORD: Admin password for authentication
 */

import { test, expect, routes } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";
import { send } from "../../utils/api-request";

/**
 * Resources the Wayfinder bundle creates, keyed by the field each list endpoint uses for its
 * array of items. Query params bump `limit` to the API's max (100) so results aren't truncated
 * by an endpoint's default page size, since some are shared with other test suites' data.
 * Matches on display `name` (or, for users, `attributes.username`) rather than the bundle's
 * internal `id`/`ouId` values, since only `name`/`username` are guaranteed to appear verbatim in
 * the list response regardless of how the backend assigns resource ids on import.
 */
const RESOURCE_MANIFEST: Array<{
  label: string;
  path: string;
  listKey: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  getName: (item: any) => string | undefined;
  expected: string[];
}> = [
  {
    label: "Applications",
    path: "/applications?limit=100",
    listKey: "applications",
    getName: a => a.name,
    expected: ["Wayfinder", "External MCP Client", "Heidi Wallet", "Lissi Wallet"],
  },
  {
    label: "Agents",
    path: "/agents?limit=100",
    listKey: "agents",
    getName: a => a.name,
    expected: ["WAYFINDER-CONCIERGE", "WAYFINDER-UPGRADE-AGENT"],
  },
  {
    label: "Users",
    path: "/users?limit=100",
    listKey: "users",
    getName: u => u.attributes?.username,
    expected: ["john.doe", "jane.smith", "alex.carter"],
  },
  {
    label: "User types",
    path: "/user-types?limit=100",
    listKey: "types",
    getName: t => t.name,
    expected: ["Customer", "Staff"],
  },
  {
    label: "Resource servers",
    path: "/resource-servers?limit=100",
    listKey: "resourceServers",
    getName: r => r.name,
    expected: ["Wayfinder Agent", "Wayfinder Booking"],
  },
  {
    label: "Roles",
    path: "/roles?limit=100",
    listKey: "roles",
    getName: r => r.name,
    expected: [
      "Traveler",
      "Support",
      "DestinationsAdmin",
      "OpsAdmin",
      "Chat User",
      "Booking User",
      "Recommender",
      "Upgrade Scheduler",
    ],
  },
  {
    label: "Flows",
    path: "/flows?limit=100",
    listKey: "flows",
    getName: f => f.name,
    expected: [
      "Wayfinder Registration Flow",
      "Wayfinder Password Recovery Flow",
      "Wayfinder Staff Onboarding Flow",
      "Default Wayfinder CIBA Email Notification Flow",
      "Default Wayfinder CIBA SMS Notification Flow",
      "Wayfinder Agent Authentication Flow",
    ],
  },
];

test.describe("Wayfinder Sample Setup", { tag: [TestTags.WAYFINDER] }, () => {
  test.describe.serial("Config Import", () => {
    /** TC001: Setup card renders with expected steps */
    test("TC001: Setup card renders with expected steps", async ({ welcomePage }) => {
      await test.step("Navigate to the Secured Web Application tryout page", async () => {
        await welcomePage.gotoTryoutApp();
      });

      await test.step("Expand the setup card and verify its steps and import button", async () => {
        await welcomePage.expandSetupCardIfCollapsed();
        await expect(welcomePage.getSampleStepTitle).toBeVisible();
        await expect(welcomePage.configureStepTitle).toBeVisible();
        await expect(welcomePage.runStepTitle).toBeVisible();
        await expect(welcomePage.importButton).toBeVisible();
        await expect(welcomePage.importButton).toBeEnabled();
        await welcomePage.screenshot("tc001-wayfinder-setup-idle");
      });
    });

    /** TC002: Download button downloads the Wayfinder sample */
    test("TC002: Download button downloads the Wayfinder sample", async ({ welcomePage }) => {
      await test.step("Open the setup card", async () => {
        await welcomePage.gotoTryoutApp();
        await welcomePage.expandSetupCardIfCollapsed();
      });

      await test.step("Trigger the download and verify it starts", async () => {
        const download = await welcomePage.triggerDownload();
        console.log("Download suggested filename:", download.suggestedFilename());
        await download.cancel().catch(() => {});
      });
    });

    /** TC003: Import completes successfully, and the already-configured state persists on revisit within the same session */
    test("TC003: Import completes and already-configured state persists on revisit", async ({ welcomePage }) => {
      await test.step("Open the setup card", async () => {
        await welcomePage.gotoTryoutApp();
        await welcomePage.expandSetupCardIfCollapsed();
      });

      await test.step("Run the import and verify it succeeds with zero failures", async () => {
        const response = await welcomePage.runImportAndWait();
        expect(response.ok(), `POST /import should succeed, got ${response.status()}`).toBe(true);

        const body = await response.json();
        expect(body.summary.failed, "no resource should fail to import").toBe(0);
        expect(body.summary.imported, "at least one resource should have been imported").toBeGreaterThan(0);

        await expect(welcomePage.importSuccessLabel).toBeVisible();
        await expect(welcomePage.importResourcesImportedLabel).toBeVisible();
        await expect(welcomePage.importErrorLabel).not.toBeVisible();
        await welcomePage.screenshot("tc003-wayfinder-import-success");
      });

      await test.step("Reload the tryout page in the same session and verify the already-configured state", async () => {
        await welcomePage.gotoTryoutApp();
        expect(await welcomePage.isWayfinderConfiguredFlagSet()).toBe(true);

        await welcomePage.expandSetupCardIfCollapsed();
        await expect(welcomePage.importAlreadyDoneLabel).toBeVisible();
        await expect(welcomePage.importLastImportedLabel).toBeVisible();
        await expect(welcomePage.reconfigureButton).toBeVisible();
        await welcomePage.screenshot("tc003-wayfinder-already-configured");
      });
    });

    /** TC004: Every resource the bundle declares is present via the admin API */
    test("TC004: All resources loaded via API", async ({ request }) => {
      for (const resource of RESOURCE_MANIFEST) {
        await test.step(resource.label, async () => {
          const response = await send(request, "GET", resource.path);
          expect(response.ok(), `GET ${resource.path} should succeed, got ${response.status()}`).toBe(true);

          const body = await response.json();
          const names = (body[resource.listKey] ?? []).map(resource.getName);
          for (const expected of resource.expected) {
            expect(names, `${resource.label} should include "${expected}"`).toContain(expected);
          }
        });
      }
    });

    // TC005 is a UI spot-check that the imported resources actually render in the console (TC004
    // already verifies the full set via the API). The applications/agents/users grids page at 10
    // rows, and Wayfinder's resources are the newest rows in each, so they can land off the first
    // page (see tests/e2e/pages/user-management/users.page.ts's gotoUserDetails doc comment for the
    // same issue). Resolving each resource's id via the admin API and navigating straight to its
    // details page avoids depending on grid pagination.
    /** TC005: Key created resources are visible in the console UI (spot check) */
    test("TC005: Key resources visible in console UI", async ({ welcomePage, request }) => {
      const page = welcomePage.page;

      /** Fetch a resource list and return the id of the item whose name matches. */
      const findId = async (label: string, name: string): Promise<string> => {
        const resource = RESOURCE_MANIFEST.find(r => r.label === label)!;
        const response = await send(request, "GET", resource.path);
        expect(response.ok(), `GET ${resource.path} should succeed, got ${response.status()}`).toBe(true);
        const body = await response.json();
        const item = (body[resource.listKey] ?? []).find((i: unknown) => resource.getName(i) === name);
        expect(item, `${resource.path} should include "${name}"`).toBeDefined();
        return item.id;
      };

      const seeOnDetailsPage = async (routePath: string, text: string): Promise<void> => {
        await page.goto(`${welcomePage.baseUrl}${routePath}`, { timeout: Timeouts.PAGE_LOAD });
        await expect(page.getByText(text, { exact: true }).first()).toBeVisible();
      };

      await test.step("Wayfinder application appears on its details page", async () => {
        const id = await findId("Applications", "Wayfinder");
        await seeOnDetailsPage(routes.applicationDetails(id), "Wayfinder");
      });

      await test.step("WAYFINDER-CONCIERGE agent appears on its details page", async () => {
        const id = await findId("Agents", "WAYFINDER-CONCIERGE");
        await seeOnDetailsPage(routes.agentDetails(id), "WAYFINDER-CONCIERGE");
      });

      await test.step("john.doe appears on their details page", async () => {
        const id = await findId("Users", "john.doe");
        await seeOnDetailsPage(routes.userDetails(id), "john.doe");
      });
    });
  });
});
