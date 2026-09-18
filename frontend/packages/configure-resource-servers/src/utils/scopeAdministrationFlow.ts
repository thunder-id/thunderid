// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';

/**
 * Routes the deletion of an action through the scope deletion flow, so the scope it defines is denied
 * before it stops existing.
 *
 * This revocation is the one that is deployment-wide rather than per principal: a retired scope should
 * be held by nobody, so there is no set of holders to enumerate, and enumerating one would miss
 * whoever obtains a token next.
 */

/**
 * Deletes an action through the configured flow, or returns false when none is configured so the
 * caller can use the native endpoint.
 *
 * The resource is optional because an action may be defined on the resource server itself rather than
 * under one of its resources, and the two are different actions to look up.
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
    AdministrationFlowConfigKey.SCOPE_DELETION,
    inputs,
    'scope deletion',
  );

  return result !== null;
}
