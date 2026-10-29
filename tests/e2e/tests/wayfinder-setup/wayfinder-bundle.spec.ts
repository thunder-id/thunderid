// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder Sample Bundle E2E Tests
 *
 * Verifies, through the admin API, that every resource the Wayfinder declarative bundle declares
 * (applications, agents, users, user types, resource servers, roles, flows, the email provider)
 * is present on the Data Plane. run-e2e.sh imports the bundle (POST /import) before this runs, and
 * the tryout specs in tests/wayfinder/** depend on the data it creates.
 *
 * Required environment variables:
 * - SERVER_URL: ThunderID server URL for direct API calls (defaults to https://localhost:8090)
 */

import { test, expect } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
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
  // Omitted for endpoints that return a bare array, such as `/connections/{vendor}`.
  listKey?: string;
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
    path: "/agents?limit=100&include=display",
    listKey: "agents",
    getName: a => a.display,
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
    getName: t => t.handle,
    expected: ["customer", "staff"],
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
      "Wayfinder Sign Out Flow",
      "Wayfinder Staff Onboarding Flow",
      "Default Wayfinder CIBA Email Notification Flow",
      "Default Wayfinder CIBA SMS Notification Flow",
      "Wayfinder Agent Authentication Flow",
    ],
  },
  {
    label: "Email providers",
    path: "/connections/email-smtp",
    getName: c => c.name,
    expected: ["Wayfinder Mock SMTP"],
  },
];

test.describe("Wayfinder Sample Bundle", { tag: [TestTags.WAYFINDER] }, () => {
  /** TC001: Every resource the bundle declares is present via the admin API */
  test("TC001: All resources loaded via API", async ({ request }) => {
    for (const resource of RESOURCE_MANIFEST) {
      await test.step(resource.label, async () => {
        const response = await send(request, "GET", resource.path);
        expect(response.ok(), `GET ${resource.path} should succeed, got ${response.status()}`).toBe(true);

        const body = await response.json();
        const names = ((resource.listKey ? body[resource.listKey] : body) ?? []).map(resource.getName);
        for (const expected of resource.expected) {
          expect(names, `${resource.label} should include "${expected}"`).toContain(expected);
        }
      });
    }
  });
});
