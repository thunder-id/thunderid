// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { mergeTests } from "@playwright/test";
import { test as sampleAppTest } from "./sample-app.fixture";
import { test as supportTest } from "../console/console-support.fixture";

// console-support.fixture is an independent branch off the raw Playwright base (same shape as
// sample-app.fixture), built to be merged in like this - see fixtures/console/index.ts.
export const test = mergeTests(sampleAppTest, supportTest);

export { expect } from "@playwright/test";
export { SampleAppLoginPage } from "../../pages/sample-app";
export { UsersApi, type ApiUser } from "../../utils/users-api";
