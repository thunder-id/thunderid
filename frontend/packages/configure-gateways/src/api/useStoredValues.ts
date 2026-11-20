// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useMutation,
  useMutationState,
  useQuery,
  useQueryClient,
  type Mutation,
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
 * The most a lookup's names may take in its query string as sent, each escaped and a `%2C` between two.
 * Many proxies refuse a request line past 8 KB, and a name may run to 255 characters, so a batch stays
 * well short of that.
 */
export const MAX_NAMES_QUERY_BYTES = 6 * 1024;

/**
 * The names split into the batches one lookup takes: at most NAMES_PER_LOOKUP names each, whose names
 * fit in MAX_NAMES_QUERY_BYTES as sent.
 */
export function nameBatches(names: readonly string[]): string[][] {
  const batches: string[][] = [];
  let batch: string[] = [];
  let size = 0;
  names.forEach((name: string) => {
    const length = encodeURIComponent(name).length;
    const cost = batch.length > 0 ? length + 3 : length;
    if (batch.length === NAMES_PER_LOOKUP || (batch.length > 0 && size + cost > MAX_NAMES_QUERY_BYTES)) {
      batches.push(batch);
      batch = [name];
      size = length;
      return;
    }
    batch.push(name);
    size += cost;
  });
  if (batch.length > 0) batches.push(batch);
  return batches;
}

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
 * The page before stays shown while the next one is read, but only for the same gateway and kind: rows
 * of another gateway would act on this one.
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
    placeholderData: (previousData, previousQuery) =>
      previousQuery?.queryKey[1] === gatewayId && previousQuery?.queryKey[2] === collectionOf(kind)
        ? previousData
        : undefined,
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
 * The variables or the secrets of the given names a gateway holds, looked up by name in the batches
 * `nameBatches` makes, one after another, rather than read in full. A name the gateway does not hold is absent from the answer, and a name no
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
      // One at a time, so a long list of names does not open many requests to the gateway at once.
      const pages = await nameBatches(wanted).reduce<Promise<StoredValuePage[]>>(
        async (read: Promise<StoredValuePage[]>, batch: string[]) => [
          ...(await read),
          await source.listValues(gatewayId, kind, {names: batch, limit: NAMES_PER_LOOKUP, offset: 0}),
        ],
        Promise.resolve([]),
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

/**
 * The key a write of one gateway's values of one kind runs under, so every view of the gateway sees a
 * write another view started: `set` or `remove`, then the gateway and the kind. The value's name is the
 * write's variables.
 */
const writeKey = (write: 'set' | 'remove', gatewayId: string, kind: ValueKind): string[] => [
  DeployQueryKeys.VALUES,
  'write',
  write,
  gatewayId,
  kind,
];

/** The writes to a gateway's values in flight in any view, each as `kind:name`. */
export interface PendingValueWrites {
  /** The values a removal is pending for: setting one now could be undone when the removal lands. */
  removing: Set<string>;
  /** The values a save is pending for: removing one now could be undone when the save lands. */
  setting: Set<string>;
}

/** How a pending write names its value, as `PendingValueWrites` holds it. */
export const pendingWriteOf = (kind: ValueKind, name: string): string => `${kind}:${name}`;

/**
 * The writes to a gateway's values pending in any view, so a view can hold back a set while a removal
 * of the same value is in flight, and a removal while a save is, wherever the other was started.
 */
export function usePendingValueWrites(gatewayId: string): PendingValueWrites {
  const removing = useMutationState({
    filters: {mutationKey: [DeployQueryKeys.VALUES, 'write', 'remove', gatewayId], status: 'pending'},
    select: pendingWriteOfMutation,
  });
  const setting = useMutationState({
    filters: {mutationKey: [DeployQueryKeys.VALUES, 'write', 'set', gatewayId], status: 'pending'},
    select: pendingWriteOfMutation,
  });
  // Each list keeps its identity while what it holds does not change.
  return useMemo(() => ({removing: new Set(removing), setting: new Set(setting)}), [removing, setting]);
}

/** A pending write to a value, as `kind:name`: the kind from its key, the name from its variables. */
function pendingWriteOfMutation(mutation: Mutation<unknown, Error, unknown>): string {
  const kind = mutation.options.mutationKey?.[4] as ValueKind;
  const variables = mutation.state.variables as string | SetValueInput | undefined;
  return pendingWriteOf(kind, typeof variables === 'string' ? variables : (variables?.name ?? ''));
}

/** Sets a variable or a secret on a gateway, creating it when the gateway does not hold it. */
export function useSetStoredValue(
  gatewayId: string,
  kind: ValueKind,
): UseMutationResult<StoredValue, Error, SetValueInput> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<StoredValue, Error, SetValueInput>({
    mutationKey: writeKey('set', gatewayId, kind),
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
    mutationKey: writeKey('remove', gatewayId, kind),
    mutationFn: (name: string): Promise<void> => source.deleteValue(gatewayId, kind, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: valuesKey(gatewayId, kind)});
    },
  });
}
