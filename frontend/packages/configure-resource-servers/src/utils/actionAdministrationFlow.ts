// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';

/**
 * Deletes an action through the configured flow, or returns false when none is configured.
 * `resourceId` is omitted for an action defined on the resource server itself.
 */
export async function deleteActionViaFlow(
  http: HttpLike,
  serverUrl: string,
  resourceServerId: string,
  actionId: string,
  resourceId?: string,
): Promise<boolean> {
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

  return result !== null;
}
