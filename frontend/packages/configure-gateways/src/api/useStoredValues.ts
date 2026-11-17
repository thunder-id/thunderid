// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import {useMemo} from 'react';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import useGatewayValueAccess from '../hooks/useGatewayValueAccess';
import type {StoredValue, StoredValuePage, ValueKind} from '../models/deploy';

const collectionOf = (kind: ValueKind): string => (kind === 'secret' ? 'secrets' : 'variables');

/** The most names one read looks up, and the largest page a gateway's store answers with. */
export const NAMES_PER_LOOKUP = 100;

/**
 * How long a read of values is taken as current. Every set and remove made here refreshes them at once,
 * so a read is not repeated on every mount or window focus.
 */
const VALUES_STALE_TIME = 30_000;

const NAME_FORMAT = /^[A-Za-z_][A-Za-z0-9_]*$/;
const MAX_NAME_LENGTH = 255;

/** Whether a name, or the start of one, is one a gateway's store can hold. */
export const isValueName = (name: string): boolean => name.length <= MAX_NAME_LENGTH && NAME_FORMAT.test(name);

/** The query key every read of a gateway's values of one kind starts with, so a change refreshes them all. */
const valuesKey = (gatewayId: string, kind: ValueKind): string[] => [
  DeployQueryKeys.VALUES,
  gatewayId,
  collectionOf(kind),
];

export interface StoredValuesPageQuery {
  limit: number;
  offset: number;
  /** Only the names that start with this. */
  prefix?: string;
}

/**
 * One page of the variables or the secrets a gateway holds, with how many match in all. A secret's value
 * is never read back. Not read for an admin who may not view them, whose request the server would refuse.
 * The page before stays shown while the next one is read.
 */
export function useStoredValuesPage(
  gatewayId: string,
  kind: ValueKind,
  {limit, offset, prefix = ''}: StoredValuesPageQuery,
  enabled = true,
): UseQueryResult<StoredValuePage> {
  const source = useDeploySource();
  const {canValue} = useGatewayValueAccess();
  return useQuery<StoredValuePage>({
    queryKey: [...valuesKey(gatewayId, kind), 'page', {limit, offset, prefix}],
    enabled: enabled && Boolean(gatewayId) && canValue(kind),
    retry: false,
    staleTime: VALUES_STALE_TIME,
    refetchOnWindowFocus: false,
    placeholderData: keepPreviousData,
    queryFn: (): Promise<StoredValuePage> =>
      source.listValues(gatewayId, kind, {limit, offset, ...(prefix ? {prefix} : {})}),
  });
}

/** How many variables or secrets a gateway holds, read as a page of one. */
export function useStoredValueCount(gatewayId: string, kind: ValueKind, enabled = true): UseQueryResult<number> {
  const source = useDeploySource();
  const {canValue} = useGatewayValueAccess();
  return useQuery<StoredValuePage, Error, number>({
    queryKey: [...valuesKey(gatewayId, kind), 'page', {limit: 1, offset: 0, prefix: ''}],
    enabled: enabled && Boolean(gatewayId) && canValue(kind),
    retry: false,
    staleTime: VALUES_STALE_TIME,
    refetchOnWindowFocus: false,
    queryFn: (): Promise<StoredValuePage> => source.listValues(gatewayId, kind, {limit: 1, offset: 0}),
    select: (page: StoredValuePage): number => page.totalResults,
  });
}

/**
 * The variables or the secrets of the given names a gateway holds, looked up by name a hundred at a time
 * rather than read in full. A name the gateway does not hold is absent from the answer, and a name no
 * store could hold is not asked for. Not read for an admin who may not view them.
 */
export function useStoredValuesNamed(
  gatewayId: string,
  kind: ValueKind,
  names: readonly string[],
  enabled = true,
): UseQueryResult<StoredValue[]> {
  const source = useDeploySource();
  const {canValue} = useGatewayValueAccess();
  const wanted = useMemo(() => [...new Set(names.filter(isValueName))].sort(), [names]);
  return useQuery<StoredValue[]>({
    queryKey: [...valuesKey(gatewayId, kind), 'named', wanted],
    enabled: enabled && Boolean(gatewayId) && canValue(kind),
    retry: false,
    staleTime: VALUES_STALE_TIME,
    refetchOnWindowFocus: false,
    queryFn: async (): Promise<StoredValue[]> => {
      const batches: string[][] = [];
      for (let start = 0; start < wanted.length; start += NAMES_PER_LOOKUP) {
        batches.push(wanted.slice(start, start + NAMES_PER_LOOKUP));
      }
      const pages = await Promise.all(
        batches.map((batch: string[]) =>
          source.listValues(gatewayId, kind, {names: batch, limit: NAMES_PER_LOOKUP, offset: 0}),
        ),
      );
      const asked = new Set(wanted);
      return pages
        .flatMap((page: StoredValuePage) => page.values)
        .filter((value: StoredValue) => asked.has(value.name));
    },
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
  return useMutation<StoredValue, Error, SetValueInput>({
    mutationFn: ({name, value, description}: SetValueInput): Promise<StoredValue> =>
      source.setValue(gatewayId, kind, name, {value, description}),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: valuesKey(gatewayId, kind)});
    },
  });
}

/** Removes a variable or a secret from a gateway. */
export function useDeleteStoredValue(gatewayId: string, kind: ValueKind): UseMutationResult<void, Error, string> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: (name: string): Promise<void> => source.deleteValue(gatewayId, kind, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: valuesKey(gatewayId, kind)});
    },
  });
}
