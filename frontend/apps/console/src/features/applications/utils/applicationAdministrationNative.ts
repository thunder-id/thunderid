// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AdministrationHttpLike} from '@thunderid/contexts';

/**
 * The management paths for application administration, reached without running anything first.
 *
 * This module is deliberately free of any flow import. It is what a Control Plane console links: a
 * plane that serves no runtime has no tokens to revoke and no sessions to detach, so the write is
 * the whole operation. Keeping the flow path in a separate module is what keeps it out of that
 * console's bundle rather than merely unreached within it.
 */

/**
 * Deletes an application through the native endpoint.
 */
export async function deleteApplicationNatively(
  http: AdministrationHttpLike,
  serverUrl: string,
  applicationId: string,
): Promise<void> {
  await http.request({
    url: `${serverUrl}/applications/${applicationId}`,
    method: 'DELETE',
    headers: {'Content-Type': 'application/json'},
  });
}
