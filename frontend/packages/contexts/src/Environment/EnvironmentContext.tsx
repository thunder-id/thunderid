// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Context, createContext} from 'react';

/**
 * A gateway this deployment applies configuration to, shown in the console as an environment.
 *
 * @public
 */
export interface Environment {
  /** The gateway's identifier. */
  id: string;
  /** The gateway's name, which is how the environment is shown. */
  name: string;
  /** Base URL the gateway serves its runtime endpoints on. */
  baseUrl?: string;
  /** Whether this is the default gateway. At most one is. */
  isDefault?: boolean;
}

/**
 * What the console shows: this deployment's configuration, where it is edited, or one environment,
 * which is the configuration as that environment's gateway runs it and is only read.
 *
 * @public
 */
export interface EnvironmentContextType {
  /** Every environment, in the order they were listed. Empty when there are none. */
  environments: Environment[];
  /** The environment shown. Undefined while the configuration is shown. */
  selected?: Environment;
  /** Shows the environment with this identifier, or the configuration when there is none. */
  select: (id?: string) => void;
}

/**
 * React context holding what the console shows for the component tree beneath it.
 *
 * The value is undefined when no `EnvironmentProvider` is present. `useEnvironment` answers that
 * with the configuration and no environments rather than reading the context directly.
 *
 * @public
 */
const EnvironmentContext: Context<EnvironmentContextType | undefined> = createContext<
  EnvironmentContextType | undefined
>(undefined);

export default EnvironmentContext;
