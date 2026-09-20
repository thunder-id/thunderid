// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowData,
  AdministrationFlowInput,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';

export type {HttpLike};

/**
 * Deletes an application through the native endpoint.
 */
export async function deleteApplicationNatively(
  http: HttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<void> {
  await http.request({
    url: `${serverUrl}/applications/${applicationId}`,
    method: 'DELETE',
    headers: {'Content-Type': 'application/json'},
  });
}

/**
 * Deletes an application through the configured deletion flow, falling back to the native endpoint.
 *
 * Going through the flow is what revokes the application's tokens and detaches its sessions before
 * the record is removed; the native endpoint does neither. A configured flow that cannot be resolved
 * refuses rather than falling back, since that is a broken opt-in, not an opt-out.
 */
export async function deleteApplicationViaFlow(
  http: HttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<void> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.APPLICATION_DELETION,
    {[AdministrationFlowInput.APPLICATION]: applicationId},
    'application deletion',
  );

  if (result === null) {
    await deleteApplicationNatively(http, serverUrl, applicationId);
  }
}

/**
 * Regenerates an application's client secret through the configured regeneration flow, returning the
 * new secret, or null when no flow is configured.
 *
 * The flow's response is the only moment the secret is readable: no read path returns it afterwards.
 */
export async function regenerateClientSecretViaFlow(
  http: HttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<string | null> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.SECRET_REGENERATION,
    {[AdministrationFlowInput.APPLICATION]: applicationId},
    'client secret regeneration',
  );

  if (result === null) {
    return null;
  }

  const secret = result[AdministrationFlowData.CLIENT_SECRET];

  if (!secret) {
    throw new Error('The client secret regeneration flow completed without returning a secret');
  }

  return secret;
}
