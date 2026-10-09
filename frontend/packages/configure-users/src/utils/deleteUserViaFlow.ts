// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  executeAdministrationFlow,
  findAdministrationFlowId,
  resolveAdministrationFlowHandle,
  type HttpLike,
} from '@thunderid/utils';

export type {HttpLike};

/**
 * Deletes a user through the native endpoint.
 */
export async function deleteUserNatively(http: HttpLike, serverUrl: string, userId: string): Promise<void> {
  await http.request({
    url: `${serverUrl}/users/${userId}`,
    method: 'DELETE',
    headers: {'Content-Type': 'application/json'},
  });
}

/**
 * Deletes a user through the configured deletion flow, falling back to the native endpoint.
 *
 * The presence of a usable flow is the switch: deletion runs through the flow when one is configured
 * and exists, and through the endpoint otherwise. This mirrors user onboarding, which falls back to
 * manual creation when its flow is unavailable, and means a deployment carrying no flow configuration
 * keeps deleting users.
 *
 * Unlike the authorization flows, a configured handle naming a missing flow falls back here too.
 *
 * Only a missing flow triggers the fallback. Failures from the flow itself propagate, so a flow that
 * exists but refuses or errors is never quietly downgraded to a deletion that skips revocation.
 */
export default async function deleteUser(http: HttpLike, serverUrl: string, userId: string): Promise<void> {
  const handle = await resolveAdministrationFlowHandle(http, serverUrl, AdministrationFlowConfigKey.USER_DELETION);

  if (handle) {
    const flowId = await findAdministrationFlowId(http, serverUrl, handle);

    if (flowId) {
      await executeAdministrationFlow(
        http,
        serverUrl,
        flowId,
        {[AdministrationFlowInput.SUBJECT]: userId},
        'user deletion',
      );
      return;
    }
  }

  await deleteUserNatively(http, serverUrl, userId);
}
