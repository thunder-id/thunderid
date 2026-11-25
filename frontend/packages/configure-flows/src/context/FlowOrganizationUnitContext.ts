// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext} from 'react';

/**
 * The ID of the organization unit that owns the flow being edited, for property panels that offer only that
 * organization unit and its descendants. Undefined outside the flow builder, or while the flow has no owner yet.
 */
const FlowOrganizationUnitContext = createContext<string | undefined>(undefined);

export default FlowOrganizationUnitContext;
