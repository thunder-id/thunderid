// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  runAdministrationFlow,
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
 * Only the flow revokes the user's grants and ends their sessions; a configured flow that cannot be
 * resolved refuses.
 */
export default async function deleteUser(http: HttpLike, serverUrl: string, userId: string): Promise<void> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.USER_DELETION,
    {[AdministrationFlowInput.SUBJECT]: userId},
    'user deletion',
  );

  if (result === null) {
    await deleteUserNatively(http, serverUrl, userId);
  }
}
