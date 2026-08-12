// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0
/**
 * Application Flow Rewiring
 *
 * Both MFASetup and SocialLoginSetup point a sample-app application at a newly created
 * authentication flow, and restore its prior flow bindings on cleanup. This is that shared
 * rewire/restore pair. Each caller passes its own clientId (see constants/sample-apps.ts) so MFA
 * and social login rewire their own dedicated application rather than contending over one shared
 * `REACT_SDK_SAMPLE`.
 */

import type { APIRequestContext } from "@playwright/test";
import { ApplicationsApi } from "../applications-api";

export type AppFlowBindings = {
  authFlowId: string | undefined;
  registrationFlowId: string | undefined;
  recoveryFlowId: string | null;
  isRegistrationFlowEnabled: boolean | undefined;
};

/**
 * Rewire the application identified by `clientId`'s flow bindings, returning its id together with
 * the bindings it had before, so a caller's cleanup can restore them via `restoreApplicationFlows`.
 */
export async function rewireApplicationFlows(
  request: APIRequestContext,
  clientId: string,
  overrides: Record<string, unknown>
): Promise<{ appId: string; originalFlows: AppFlowBindings }> {
  const applicationsApi = new ApplicationsApi(request);
  const targetApp = await applicationsApi.findByClientId(clientId);
  if (!targetApp) {
    throw new Error(`Application with clientId "${clientId}" not found`);
  }

  const appData = await applicationsApi.get(targetApp.id);
  const originalFlows: AppFlowBindings = {
    authFlowId: appData.authFlowId,
    registrationFlowId: appData.registrationFlowId,
    recoveryFlowId: appData.recoveryFlowId ?? null,
    isRegistrationFlowEnabled: appData.isRegistrationFlowEnabled,
  };

  await applicationsApi.update(targetApp.id, { ...appData, ...overrides });

  return { appId: targetApp.id, originalFlows };
}

/**
 * Restore an application's flow bindings to what they were before setup rewired them.
 */
export async function restoreApplicationFlows(
  request: APIRequestContext,
  appId: string,
  originalFlows: AppFlowBindings
): Promise<void> {
  try {
    const applicationsApi = new ApplicationsApi(request);
    const appData = await applicationsApi.get(appId);
    await applicationsApi.update(appId, { ...appData, ...originalFlows });
    console.log(`✓ Application flows reverted: ${appId}`);
  } catch (error) {
    console.log(`⚠️  Error reverting application flows: ${error}`);
  }
}
