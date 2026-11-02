// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, type Context} from 'react';
import type {EdgeInput} from '../utils/calculateEdgePath';

/**
 * Registry where each rendered edge reports the exact endpoint geometry React
 * Flow computed for it, so {@link EdgePathsProvider} can route all edges
 * together (with overlap separation) from the same coordinates the edges
 * would use individually.
 */
export interface EdgeGeometryRegistry {
  register: (input: EdgeInput) => void;
  unregister: (id: string) => void;
}

const EdgeGeometryContext: Context<EdgeGeometryRegistry | null> = createContext<EdgeGeometryRegistry | null>(null);

EdgeGeometryContext.displayName = 'EdgeGeometryContext';

export default EdgeGeometryContext;
