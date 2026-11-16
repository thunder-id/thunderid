// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries} from '@tanstack/react-query';
import {
  useRuntimeEnvironments,
  type EnvironmentValue,
  type RuntimeEnvironment,
  type RuntimeEnvironmentsContextType,
} from '@thunderid/contexts';
import {useState} from 'react';
import ApplicationQueryKeys from '../constants/application-query-keys';

/**
 * The resource whose values are read in each environment.
 *
 * @public
 */
export interface ResourceEnvironmentsOptions {
  /** The resource's type, as a deploy names it, such as `application` or `agent`. */
  resourceType: string;
  resourceId: string;
  /** Whether the resource has a client ID to read. Without one, where it is deployed is not known. */
  hasClientId: boolean;
}

/**
 * The environments a resource's overview chooses between, and what the chosen one holds.
 *
 * @public
 */
export interface ResourceEnvironments {
  environments: RuntimeEnvironment[];
  /** The environment chosen, where there are several to choose between. */
  selectedEnvironment?: RuntimeEnvironment;
  /** The environment whose values are shown: the one chosen, or the only one. */
  shownEnvironment?: RuntimeEnvironment;
  selectEnvironment: (environmentId: string) => void;
  /** The environments the resource is not deployed to yet. */
  notDeployedIds: string[];
  /** Whether the resource is not deployed to the environment shown. */
  isNotDeployed: boolean;
  /** Whether the environment shown is reached by promoting from the one before it, not by a deploy. */
  isPromoted: boolean;
  /** The environment the one shown is promoted from. */
  previousEnvironmentName?: string;
  /** The client ID the environment shown holds, once read. */
  selectedClientId?: EnvironmentValue;
  getValue?: RuntimeEnvironmentsContextType['getValue'];
  /** Where configuration is deployed to the environments. */
  deployPath?: string;
}

/**
 * React hook for an overview whose client ID and endpoints differ in each environment the resource is
 * deployed to. With several environments one is chosen, the first in the pipeline until another is. With a
 * single environment there is nothing to choose, but the resource may still not be in it.
 *
 * @param options - The resource whose values are read
 * @returns The environments, the one chosen, and where the resource is not deployed yet
 *
 * @public
 */
export default function useResourceEnvironments({
  resourceType,
  resourceId,
  hasClientId,
}: ResourceEnvironmentsOptions): ResourceEnvironments {
  const runtimeEnvironments = useRuntimeEnvironments();
  const environments: RuntimeEnvironment[] = runtimeEnvironments?.environments ?? [];
  const [chosenEnvironmentId, setChosenEnvironmentId] = useState<string>();
  const selectedEnvironment: RuntimeEnvironment | undefined =
    environments.length > 1
      ? (environments.find((environment) => environment.id === chosenEnvironmentId) ?? environments[0])
      : undefined;
  const shownEnvironment: RuntimeEnvironment | undefined =
    selectedEnvironment ?? (environments.length === 1 ? environments[0] : undefined);
  const shownIndex = environments.findIndex((environment) => environment.id === shownEnvironment?.id);

  // Which environments the resource is not deployed to is told on the picker, before one is chosen. It is
  // read the way each environment's client ID is, under the same key, so neither is asked for twice.
  const clientIdByEnvironment = useQueries({
    queries: (shownEnvironment && hasClientId ? environments : []).map((environment: RuntimeEnvironment) => ({
      queryKey: [ApplicationQueryKeys.ENVIRONMENT_VALUE, environment.id, resourceId, 'clientId'],
      queryFn: (): Promise<EnvironmentValue> =>
        runtimeEnvironments!.getValue(environment.id, {resourceType, resourceId, field: 'clientId'}),
      retry: false,
    })),
  });
  const notDeployedIds = environments
    .filter(
      (_environment: RuntimeEnvironment, index: number) => clientIdByEnvironment[index]?.data?.status === 'absent',
    )
    .map((environment: RuntimeEnvironment) => environment.id);

  return {
    environments,
    selectedEnvironment,
    shownEnvironment,
    selectEnvironment: setChosenEnvironmentId,
    notDeployedIds,
    isNotDeployed: Boolean(shownEnvironment) && notDeployedIds.includes(shownEnvironment!.id),
    // A deploy enters at the first environment; each later one is reached by promoting.
    isPromoted: shownIndex > 0,
    previousEnvironmentName: environments[shownIndex - 1]?.name,
    selectedClientId: shownEnvironment ? clientIdByEnvironment[shownIndex]?.data : undefined,
    getValue: runtimeEnvironments?.getValue,
    deployPath: runtimeEnvironments?.deployPath,
  };
}
