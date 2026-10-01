// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext} from 'react';

/**
 * Starts the end-user flow preview focused directly at a node, so a single
 * screen can be previewed from the canvas. Undefined outside a host that
 * provides the preview (e.g. isolated node renders in tests).
 */
const StepPreviewContext = createContext<((nodeId: string) => void) | undefined>(undefined);

export default StepPreviewContext;
