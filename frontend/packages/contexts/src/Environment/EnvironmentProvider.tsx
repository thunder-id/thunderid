// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo, type JSX, type PropsWithChildren} from 'react';
import EnvironmentContext, {type Environment, type EnvironmentContextType} from './EnvironmentContext';

/**
 * Props for the EnvironmentProvider component.
 *
 * @public
 */
export interface EnvironmentProviderProps extends PropsWithChildren {
  /** Every environment there is. */
  environments: Environment[];
  /** The identifier of the environment shown, absent while the configuration is shown. */
  selectedId?: string;
  /** Called with the identifier of the environment chosen, or with none for the configuration. */
  onSelect: (id?: string) => void;
}

/**
 * Tells the console whether it shows this deployment's configuration or one environment.
 *
 * A control plane holds one configuration for every gateway it applies configuration to. The
 * configuration is where it is edited. An environment is that configuration as one gateway runs it,
 * from the version the gateway applied, and is only read. The choice is the host application's:
 * the tree below reads it through `useEnvironment`.
 *
 * @public
 */
export default function EnvironmentProvider({
  environments,
  selectedId = undefined,
  onSelect,
  children,
}: EnvironmentProviderProps): JSX.Element {
  const value: EnvironmentContextType = useMemo(
    () => ({
      environments,
      selected: environments.find((environment: Environment) => environment.id === selectedId),
      select: onSelect,
    }),
    [environments, selectedId, onSelect],
  );

  return <EnvironmentContext.Provider value={value}>{children}</EnvironmentContext.Provider>;
}
