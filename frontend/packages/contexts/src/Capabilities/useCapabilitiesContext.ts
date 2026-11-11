// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import CapabilitiesContext, {type Capabilities} from './CapabilitiesContext';

/**
 * Reads what the signed-in admin may do. Without a `CapabilitiesProvider`, nothing is allowed.
 *
 * @public
 */
export default function useCapabilitiesContext(): Capabilities {
  return useContext(CapabilitiesContext);
}
