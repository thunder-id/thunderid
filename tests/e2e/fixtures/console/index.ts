/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * Console Combined Fixture
 *
 * Merges routes and POM fixtures into a single test export.
 * Use this as the primary import for tests.
 */

import { mergeTests } from "@playwright/test";
import { test as routesTest, routes, ConsoleRoutes } from "./console-routes.fixture";
import { test as pomTest } from "./console-pom.fixture";

/**
 * Combined test fixture.
 * Note: pomTest already extends auth fixture, so authentication fixtures are included here.
 */
export const test = mergeTests(routesTest, pomTest);
export const setup = test;

export { expect } from "@playwright/test";
export { routes, ConsoleRoutes };

// Re-export page objects
export { ConsoleSigninPage } from "../../pages/authentication";
export { UsersPage, type UserFormData } from "../../pages/user-management";
export { ApplicationsPage, type ApplicationFormData } from "../../pages/applications";
export { SettingsPage } from "../../pages/settings";
export { WelcomePage } from "../../pages/welcome";
