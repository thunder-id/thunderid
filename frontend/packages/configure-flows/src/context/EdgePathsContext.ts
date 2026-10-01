// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, type Context} from 'react';
import type {EdgePathResult} from '../utils/calculateEdgePath';

/**
 * Separated (lane-offset) paths for every edge on the canvas, keyed by edge id.
 * `null` while a node is being dragged — edges then fall back to their cheap
 * per-edge paths. Populated by {@link EdgePathsProvider}.
 */
const EdgePathsContext: Context<Map<string, EdgePathResult> | null> = createContext<Map<string, EdgePathResult> | null>(
  null,
);

EdgePathsContext.displayName = 'EdgePathsContext';

export default EdgePathsContext;
