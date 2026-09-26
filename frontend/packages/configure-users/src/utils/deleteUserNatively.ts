// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AdministrationHttpLike} from '@thunderid/contexts';

/**
 * Deletes a user through the native endpoint.
 *
 * This module is deliberately free of any flow import, so a console that only ever deletes this way
 * does not carry the flow path in its bundle. A Control Plane is that console: it serves no runtime,
 * so there are no sessions to detach before the record goes.
 */
export default async function deleteUserNatively(
  http: AdministrationHttpLike,
  serverUrl: string,
  userId: string,
): Promise<void> {
  await http.request({
    url: `${serverUrl}/users/${userId}`,
    method: 'DELETE',
    headers: {'Content-Type': 'application/json'},
  });
}
