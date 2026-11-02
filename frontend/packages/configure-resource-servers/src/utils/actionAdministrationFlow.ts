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
 * Deletes an action through the native endpoint. `resourceId` is omitted for an action defined on the
 * resource server itself.
 */
export async function deleteActionNatively(
  http: HttpLike,
  serverUrl: string,
  resourceServerId: string,
  actionId: string,
  resourceId?: string,
): Promise<void> {
  const url = resourceId
    ? `${serverUrl}/resource-servers/${resourceServerId}/resources/${resourceId}/actions/${actionId}`
    : `${serverUrl}/resource-servers/${resourceServerId}/actions/${actionId}`;

  await http.request({
    url,
    method: 'DELETE',
  });
}

/**
 * Deletes an action through the configured deletion flow, falling back to the native endpoint.
 * `resourceId` is omitted for an action defined on the resource server itself.
 *
 * Only the flow revokes the scope the action defines; a configured flow that cannot be resolved refuses.
 */
export async function deleteActionViaFlow(
  http: HttpLike,
  serverUrl: string,
  resourceServerId: string,
  actionId: string,
  resourceId?: string,
): Promise<void> {
  const inputs: Record<string, string> = {
    [AdministrationFlowInput.ACTION]: actionId,
    [AdministrationFlowInput.RESOURCE_SERVER]: resourceServerId,
  };

  if (resourceId) {
    inputs[AdministrationFlowInput.RESOURCE] = resourceId;
  }
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.ACTION_DELETION,
    inputs,
    'action deletion',
  );

  if (result === null) {
    await deleteActionNatively(http, serverUrl, resourceServerId, actionId, resourceId);
  }
}
