// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import type {StoredValue, ValueKind} from '../models/deploy';

const collectionOf = (kind: ValueKind): string => (kind === 'secret' ? 'secrets' : 'variables');

/** Every variable or every secret a gateway holds. A secret's value is never read back. */
export function useStoredValues(gatewayId: string, kind: ValueKind, enabled = true): UseQueryResult<StoredValue[]> {
  const source = useDeploySource();
  const collection = collectionOf(kind);
  return useQuery<StoredValue[]>({
    queryKey: [DeployQueryKeys.VALUES, gatewayId, collection],
    enabled: enabled && Boolean(gatewayId),
    retry: false,
    queryFn: (): Promise<StoredValue[]> => source.values(gatewayId, kind),
  });
}

export interface SetValueInput {
  name: string;
  value: string;
  /** The value's description, kept as it is when given. */
  description?: string;
}

/** Sets a variable or a secret on a gateway, creating it when the gateway does not hold it. */
export function useSetStoredValue(
  gatewayId: string,
  kind: ValueKind,
): UseMutationResult<StoredValue, Error, SetValueInput> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  const collection = collectionOf(kind);
  return useMutation<StoredValue, Error, SetValueInput>({
    mutationFn: ({name, value, description}: SetValueInput): Promise<StoredValue> =>
      source.setValue(gatewayId, kind, name, {value, description}),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.VALUES, gatewayId, collection]});
    },
  });
}

/** Removes a variable or a secret from a gateway. */
export function useDeleteStoredValue(gatewayId: string, kind: ValueKind): UseMutationResult<void, Error, string> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  const collection = collectionOf(kind);
  return useMutation<void, Error, string>({
    mutationFn: (name: string): Promise<void> => source.deleteValue(gatewayId, kind, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.VALUES, gatewayId, collection]});
    },
  });
}
