// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {HttpLike} from '@thunderid/utils';

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
 * Regenerates an application's client secret through the native endpoint and returns the new secret.
 *
 * The response is the only moment the secret is readable: no read path returns it afterwards.
 */
export async function regenerateClientSecretNatively(
  http: HttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<{clientSecret: string; revocationIncomplete: boolean}> {
  const response = await http.request({
    url: `${serverUrl}/applications/${applicationId}/client-secret/regenerate`,
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
  });
  const data = response.data as {clientSecret?: string; revocationIncomplete?: boolean} | undefined;

  if (!data?.clientSecret) {
    throw new Error('The client secret regeneration completed without returning a secret');
  }

  return {clientSecret: data.clientSecret, revocationIncomplete: data.revocationIncomplete === true};
}
