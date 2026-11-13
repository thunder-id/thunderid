// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX, PropsWithChildren} from 'react';
import HiddenResourcesContext, {type HiddenResources} from './HiddenResourcesContext';

/**
 * Props for the HiddenResourcesProvider component.
 *
 * @public
 */
export interface HiddenResourcesProviderProps extends PropsWithChildren {
  /** The resources the feature packages beneath it leave out of their views. */
  hidden: HiddenResources;
}

/**
 * Declares the resources the feature packages beneath it leave out of their views.
 *
 * @example
 * ```tsx
 * <HiddenResourcesProvider hidden={{applicationClientIds: ['ADMIN_CONSOLE']}}>
 *   <Routes />
 * </HiddenResourcesProvider>
 * ```
 *
 * @public
 */
export default function HiddenResourcesProvider({hidden, children}: HiddenResourcesProviderProps): JSX.Element {
  return <HiddenResourcesContext.Provider value={hidden}>{children}</HiddenResourcesContext.Provider>;
}
