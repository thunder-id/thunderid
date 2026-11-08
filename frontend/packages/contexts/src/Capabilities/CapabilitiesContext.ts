// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Context, createContext} from 'react';

/**
 * What the signed-in admin may do, used to decide which pages and actions the Console offers.
 *
 * @public
 */
export interface Capabilities {
  /**
   * Whether the admin may perform `action` (`view`, `create`, `update`, `delete`) on the catalog `resource`
   * (`app`, `user`, ...). False while loading. True when the capabilities can't be read, since the server
   * still decides every request.
   */
  can: (resource: string, action?: string) => boolean;
  /** Whether the admin may manage the APIs only the root `system` permission reaches. */
  canManageRoot: boolean;
  isLoading: boolean;
}

/**
 * React context carrying what the signed-in admin may do.
 *
 * Defaults to allowing everything, so a feature package renders standalone, in a test or under a host that
 * doesn't read capabilities, with every action shown. The server still enforces each request.
 *
 * @public
 */
const CapabilitiesContext: Context<Capabilities> = createContext<Capabilities>({
  can: () => true,
  canManageRoot: true,
  isLoading: false,
});

export default CapabilitiesContext;
