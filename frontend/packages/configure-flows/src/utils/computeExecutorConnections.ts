// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  IdentityProviderTypes,
  type BasicIdentityProvider,
  type ConnectionInstance,
} from '@thunderid/configure-connections';
import type {ExecutorConnectionInterface} from '../models/metadata';
import {ExecutionTypes} from '../models/steps';

/**
 * Mapping from IDP types to executor names.
 */
const IDP_TYPE_TO_EXECUTOR: Record<string, string> = {
  [IdentityProviderTypes.GOOGLE]: ExecutionTypes.GoogleFederation,
  [IdentityProviderTypes.GITHUB]: ExecutionTypes.GithubFederation,
};

export interface ComputeExecutorConnectionsParams {
  identityProviders?: BasicIdentityProvider[];
  smsProviders?: ConnectionInstance[];
}

/**
 * Computes executor connections from identity providers and SMS providers.
 * Groups connections by their corresponding executor type.
 *
 * @param params - Object containing identity providers and SMS providers
 * @returns Array of executor connections with their associated IDs
 */
const computeExecutorConnections = (params: ComputeExecutorConnectionsParams): ExecutorConnectionInterface[] => {
  const {identityProviders, smsProviders} = params;

  const executorMap = new Map<string, string[]>();

  // Process identity providers (for Google, GitHub, etc.)
  if (identityProviders && identityProviders.length > 0) {
    identityProviders.forEach((idp) => {
      const executorName = IDP_TYPE_TO_EXECUTOR[idp.type];

      if (executorName) {
        const existingConnections = executorMap.get(executorName) ?? [];
        // Use idp.id since the executor expects idpId (the unique identifier)
        executorMap.set(executorName, [...existingConnections, idp.id]);
      }
    });
  }

  // Process SMS providers (for SMS executor)
  if (smsProviders && smsProviders.length > 0) {
    const providerIds = smsProviders.map((provider) => provider.id);
    executorMap.set(ExecutionTypes.SMSExecutor, providerIds);
  }

  return Array.from(executorMap.entries()).map(([executorName, connections]) => ({
    executorName,
    connections,
  }));
};

export default computeExecutorConnections;
