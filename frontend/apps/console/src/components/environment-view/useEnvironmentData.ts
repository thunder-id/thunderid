// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {AppliedConfiguration, StoredValue, ValueKind} from './models';

/** How many values a gateway's store answers with in one page. */
const PAGE_SIZE = 100;

export const EnvironmentQueryKeys = {
  APPLIED_CONFIGURATION: 'environment-applied-configuration',
  VALUES: 'environment-values',
} as const;

const collectionOf = (kind: ValueKind): string => (kind === 'secret' ? 'secrets' : 'variables');

/**
 * Reads the configuration a gateway runs: every resource of the version it last applied, each as
 * the resource's own read returns it. The whole environment view is built from this one read.
 */
export function useAppliedConfiguration(gatewayId: string): UseQueryResult<AppliedConfiguration> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AppliedConfiguration>({
    queryKey: [EnvironmentQueryKeys.APPLIED_CONFIGURATION, gatewayId],
    // An apply changes it, so it is read again after a while rather than kept for the session.
    staleTime: 30_000,
    retry: false,
    queryFn: async (): Promise<AppliedConfiguration> => {
      const response: {data: AppliedConfiguration} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/applied-configuration`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);
      return response.data;
    },
  });
}

/** Reads every variable or every secret a gateway holds, page by page. */
export function useStoredValues(gatewayId: string, kind: ValueKind): UseQueryResult<StoredValue[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const collection: string = collectionOf(kind);

  return useQuery<StoredValue[]>({
    queryKey: [EnvironmentQueryKeys.VALUES, gatewayId, collection],
    retry: false,
    queryFn: async (): Promise<StoredValue[]> => {
      const values: StoredValue[] = [];
      for (;;) {
        const response: {data: Record<string, unknown>} = await http.request({
          url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/${collection}?limit=${PAGE_SIZE}&offset=${values.length}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);
        const page: StoredValue[] = (response.data?.[collection] as StoredValue[] | undefined) ?? [];
        values.push(...page);
        if (page.length === 0 || values.length >= Number(response.data?.totalResults ?? values.length)) {
          return values;
        }
      }
    },
  });
}

/** What a value is set to on a gateway. */
export interface SetValueRequest {
  name: string;
  value: string;
  /** The value's description, kept as it is. */
  description?: string;
}

/** Sets a variable or a secret on a gateway, creating it when the gateway does not hold it. */
export function useSetStoredValue(
  gatewayId: string,
  kind: ValueKind,
): UseMutationResult<unknown, Error, SetValueRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const collection: string = collectionOf(kind);

  return useMutation<unknown, Error, SetValueRequest>({
    mutationFn: async ({name, value, description}: SetValueRequest): Promise<unknown> => {
      const response: {data: unknown} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/${collection}/${encodeURIComponent(name)}`,
        method: 'PUT',
        data: {value, description},
      } as unknown as Parameters<typeof http.request>[0]);
      return response.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: [EnvironmentQueryKeys.VALUES, gatewayId, collection]});
    },
  });
}
