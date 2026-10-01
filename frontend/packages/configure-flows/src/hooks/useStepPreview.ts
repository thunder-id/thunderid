// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import StepPreviewContext from '../context/StepPreviewContext';

/**
 * Hook giving canvas nodes access to the per-step preview trigger, when the
 * hosting flow builder provides one.
 *
 * @returns The preview trigger, or undefined when unavailable.
 */
const useStepPreview = (): ((nodeId: string) => void) | undefined => useContext(StepPreviewContext);

export default useStepPreview;
