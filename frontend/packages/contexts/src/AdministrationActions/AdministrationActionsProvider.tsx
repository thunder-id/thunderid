// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX, ReactNode} from 'react';
import AdministrationActionsContext, {type AdministrationActions} from './AdministrationActionsContext';

/**
 * Props for {@link AdministrationActionsProvider}.
 */
export interface AdministrationActionsProviderProps {
  /**
   * The actions this console performs. Pass a value defined at module scope: it is handed to the
   * context unchanged, so an object built inline would be a new value on every render.
   */
  actions: AdministrationActions;
  children: ReactNode;
}

/**
 * Installs the administrative actions a console performs.
 *
 * A console that installs nothing takes the plain management path for every operation, which is what
 * a Control Plane wants: it runs no flows, so there is nothing to run before a write.
 */
export default function AdministrationActionsProvider({
  actions,
  children,
}: AdministrationActionsProviderProps): JSX.Element {
  return <AdministrationActionsContext.Provider value={actions}>{children}</AdministrationActionsContext.Provider>;
}
