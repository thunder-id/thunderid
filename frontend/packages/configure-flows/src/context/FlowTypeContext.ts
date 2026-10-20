// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext} from 'react';

/**
 * The type of the flow being edited (for example `AUTHENTICATION`), for property panels that only apply to some
 * flow types. Undefined outside the flow builder, such as in isolated renders in tests.
 */
const FlowTypeContext = createContext<string | undefined>(undefined);

export default FlowTypeContext;
