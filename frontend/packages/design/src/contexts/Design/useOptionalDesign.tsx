// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import DesignContext, {type DesignContextType} from './DesignContext';

/**
 * Reads the design configuration when a `DesignProvider` is mounted, and returns `undefined` when none is.
 *
 * For components that also render on pages without a `DesignProvider`, such as the flow adapters on console
 * pages. Use `useDesign` where the provider is required.
 */
export default function useOptionalDesign(): DesignContextType | undefined {
  return useContext(DesignContext);
}
