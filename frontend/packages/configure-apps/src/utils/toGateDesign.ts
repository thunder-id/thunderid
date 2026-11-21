// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppGateDesign} from '../models/builder';

/**
 * Reads the Gate design off an application, where an empty identifier means none is set.
 */
const toGateDesign = ({layoutId, themeId}: AppGateDesign): AppGateDesign => ({
  layoutId: layoutId === '' ? undefined : layoutId,
  themeId: themeId === '' ? undefined : themeId,
});

export default toGateDesign;
