// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX, PropsWithChildren} from 'react';
import CapabilitiesContext, {type Capabilities} from './CapabilitiesContext';

/**
 * Props for the CapabilitiesProvider component.
 *
 * @public
 */
export interface CapabilitiesProviderProps extends PropsWithChildren {
  /** What the signed-in admin may do, as the host app read it. */
  capabilities: Capabilities;
}

/**
 * Supplies the signed-in admin's capabilities to the feature packages beneath it, so they offer only the
 * actions the admin may perform.
 *
 * @public
 */
export default function CapabilitiesProvider({capabilities, children}: CapabilitiesProviderProps): JSX.Element {
  return <CapabilitiesContext.Provider value={capabilities}>{children}</CapabilitiesContext.Provider>;
}
