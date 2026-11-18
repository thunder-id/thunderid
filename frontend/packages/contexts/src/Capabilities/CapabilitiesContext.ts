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
   * (`app`, `user`, `export`, ...). False while loading, when the capabilities can't be read, and for a
   * resource or action the server doesn't report.
   */
  can: (resource: string, action?: string) => boolean;
  isLoading: boolean;
  /** Why the capabilities couldn't be read, so a page can say so instead of reporting a denial. */
  error?: Error | null;
  /** Reads the capabilities again after `error`. */
  retry?: () => void;
}

/**
 * React context carrying what the signed-in admin may do.
 *
 * Defaults to allowing nothing, so a page rendered under a host that doesn't read capabilities offers no
 * action the admin may not perform.
 *
 * @public
 */
const CapabilitiesContext: Context<Capabilities> = createContext<Capabilities>({
  can: () => false,
  isLoading: false,
});

export default CapabilitiesContext;
