// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, useContext} from 'react';

/**
 * Where a resource is along the deploy path:
 * - `notDeployed`: the first target has never received it.
 * - `changed`: the first target runs an older form of it.
 * - `notPromoted`: the first target runs it, and `target` does not, or runs an older form.
 * - `refused`: `target` refused it when it was last deployed there.
 * - `live`: every target runs it as it is.
 */
export type DeployState = 'notDeployed' | 'changed' | 'notPromoted' | 'refused' | 'live';

export interface DeployStatus {
  state: DeployState;
  /** The target the next step reaches: the first target to deploy to, or the one to promote into. */
  target?: string;
  /** Why `target` refused it, when it did. */
  reason?: string;
}

export interface DeployStatusSource {
  /** Where a resource is along the deploy path; undefined while it is not known. */
  statusOf: (resourceType: string, id: string) => DeployStatus | undefined;
  /** Opens where the next step is taken. */
  openDeploy: () => void;
  /** Opens the next step for one resource alone, such as a promotion of just it; the deploy path otherwise. */
  openResource?: (resourceType: string, id: string, target?: string) => void;
}

const DeployStatusContext = createContext<DeployStatusSource | null>(null);

export const DeployStatusProvider = DeployStatusContext.Provider;

/** The deploy status source the app provides; null in an app that deploys nothing. */
export function useDeployStatusSource(): DeployStatusSource | null {
  return useContext(DeployStatusContext);
}
