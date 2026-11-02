// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX, PropsWithChildren} from 'react';
import AdministrationContext, {type AdministrationConfig} from './AdministrationContext';

/**
 * Props for the AdministrationProvider component.
 *
 * @public
 */
export interface AdministrationProviderProps extends PropsWithChildren {
  /** How administration operations run beneath this provider. */
  administration: AdministrationConfig;
}

/**
 * Declares how operations that change state are carried out by the feature packages beneath it.
 *
 * This is the shape a shell will eventually express through feature composition. Until that
 * exists, a console supplies it here, which lets a package adopt the option now without any of
 * them learning which plane they are running on.
 *
 * @example
 * A console whose deployment holds configuration only, so nothing has sessions to clean up,
 * except for users, whose deletion still runs through the flow:
 * ```tsx
 * <AdministrationProvider
 *   administration={{
 *     mode: AdministrationModes.NATIVE,
 *     features: {users: {operations: {delete: AdministrationModes.FLOW}}},
 *   }}
 * >
 *   <Routes />
 * </AdministrationProvider>
 * ```
 *
 * @public
 */
export default function AdministrationProvider({administration, children}: AdministrationProviderProps): JSX.Element {
  return <AdministrationContext.Provider value={administration}>{children}</AdministrationContext.Provider>;
}
