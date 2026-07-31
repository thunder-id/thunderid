// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Console Combined Fixture
 *
 * Merges routes and POM fixtures into a single test export.
 * Use this as the primary import for tests.
 */

import { mergeTests } from "@playwright/test";
import { test as routesTest, routes, ConsoleRoutes } from "./console-routes.fixture";
import { test as pomTest } from "./console-pom.fixture";
import { test as supportTest } from "./console-support.fixture";

/**
 * Combined test fixture.
 * Note: pomTest already extends auth fixture, so authentication fixtures are included here.
 */
export const test = mergeTests(routesTest, pomTest, supportTest);
export const setup = test;

export { expect } from "@playwright/test";
export { routes, ConsoleRoutes };

// Re-export page objects
export { ConsoleSigninPage } from "../../pages/authentication";
export { UsersPage, type UserFormData } from "../../pages/user-management";
export { UserTypesPage } from "../../pages/user-types";
export { ApplicationsPage, type ApplicationFormData } from "../../pages/applications";
export { SettingsPage } from "../../pages/settings";

// Re-export API helpers so hooks can construct them without reaching into utils/
export { UsersApi, type ApiUser } from "../../utils/users-api";
export { UserTypesApi, type ApiUserType } from "../../utils/user-types-api";
export { ApplicationsApi, type ApiApplication } from "../../utils/applications-api";
