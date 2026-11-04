// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries, type UseQueryResult} from '@tanstack/react-query';
import {useEffect} from 'react';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {useThunderID} from '@thunderid/react';
import useGetUserTypes from './useGetUserTypes';
import UserTypeQueryKeys from '../constants/userTypeQueryKeys';
import type {ApiUserType} from '../types/user-types';
import flattenSchemaAttributes, {type FlattenedAttribute} from '../utils/flattenSchemaAttributes';

/**
 * Matches the server's maximum page size, so a single list request covers every user type in
 * all realistic deployments. Paging beyond this would multiply the per schema fan-out below, so
 * the overflow is reported rather than fetched; suggestions are not an exhaustive list and custom
 * values remain allowed.
 */
const USER_TYPE_FETCH_LIMIT = 100;

/**
 * User type schemas change rarely, and the flow builder property panel remounts on every
 * element selection. Without an explicit stale time the console's default of 0 would refetch
 * the whole fan-out on each click.
 */
const ATTRIBUTE_STALE_TIME: number = 5 * 60 * 1000;

/**
 * An attribute aggregated across every user type that declares it.
 */
export interface AggregatedUserTypeAttribute extends FlattenedAttribute {
  /** Display names of the user types declaring this attribute. */
  userTypes: string[];
}

/**
 * Options for {@link useGetUserTypeAttributes}.
 */
export interface UseGetUserTypeAttributesOptions {
  /**
   * Whether to fetch. Defaults to true. When false nothing is requested, and the result is empty
   * and not loading.
   */
  enabled?: boolean;
}

/**
 * Result of {@link useGetUserTypeAttributes}.
 */
export interface UseGetUserTypeAttributesResult {
  attributes: AggregatedUserTypeAttribute[];
  isLoading: boolean;
  /**
   * The failure of the list request, or else of the first schema request that failed. Attributes
   * from the user types that did load are still returned alongside it.
   */
  error: Error | null;
  /** Retries what failed: the list request if it failed, otherwise every failed schema request. */
  refetch: () => void;
}

/**
 * Aggregate the schemas of the given user types into a de-duplicated, sorted attribute list.
 *
 * @param results - Per user type query results.
 * @returns The aggregated attributes.
 */
function aggregateAttributes(results: UseQueryResult<ApiUserType>[]): AggregatedUserTypeAttribute[] {
  const aggregated = new Map<string, AggregatedUserTypeAttribute>();

  for (const {data: userType} of results) {
    if (!userType) {
      continue;
    }

    for (const {attribute, credential} of flattenSchemaAttributes(userType.schema)) {
      // User types can disagree on whether an attribute is a credential, and each variant belongs
      // to a different field control, so they are kept as separate suggestions.
      const key = `${credential ? 'credential' : 'standard'}:${attribute}`;
      const existing = aggregated.get(key);

      if (existing) {
        if (!existing.userTypes.includes(userType.displayName)) {
          existing.userTypes.push(userType.displayName);
        }
      } else {
        aggregated.set(key, {attribute, credential, userTypes: [userType.displayName]});
      }
    }
  }

  return Array.from(aggregated.values()).sort((a, b) => a.attribute.localeCompare(b.attribute));
}

/**
 * Fetch every user type and aggregate their schemas into a single list of attributes.
 *
 * The user type applicable to a given runtime request is not known when an administrator is
 * authoring a flow, so consumers use this as a suggestion list rather than a closed set of
 * choices.
 *
 * The list endpoint omits schema bodies, so each user type's schema is fetched individually.
 * Those per-type queries deliberately reuse the key and request shape of `useGetUserType` so the
 * cache is shared with other consumers rather than duplicated.
 *
 * @param options - Optional settings, see {@link UseGetUserTypeAttributesOptions}
 * @returns The aggregated attributes, the combined loading state, any read failure and a retry.
 */
export default function useGetUserTypeAttributes(
  options?: UseGetUserTypeAttributesOptions,
): UseGetUserTypeAttributesResult {
  const {enabled = true} = options ?? {};
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const logger = useLogger('useGetUserTypeAttributes');

  const {
    data: userTypesData,
    isLoading: isLoadingList,
    error: listError,
    refetch: refetchList,
  } = useGetUserTypes({limit: USER_TYPE_FETCH_LIMIT}, {enabled});

  const totalUserTypes: number = userTypesData?.totalResults ?? 0;
  useEffect(() => {
    if (enabled && totalUserTypes > USER_TYPE_FETCH_LIMIT) {
      logger.warn('Attribute suggestions cover only the first user types returned by the server', {
        fetched: USER_TYPE_FETCH_LIMIT,
        totalResults: totalUserTypes,
      });
    }
  }, [enabled, totalUserTypes, logger]);

  // A disabled list may still hold data cached by another consumer; its types must not be fetched.
  const {attributes, isLoadingSchemas, schemaError, failedSchemaRefetches} = useQueries({
    queries: (enabled ? (userTypesData?.types ?? []) : []).map((userType) => ({
      queryKey: [UserTypeQueryKeys.USER_TYPE, userType.id],
      queryFn: async (): Promise<ApiUserType> => {
        const serverUrl: string = getServerUrl();

        const response: {
          data: ApiUserType;
        } = await http.request({
          url: `${serverUrl}/user-types/${userType.id}?include=display`,
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
        } as unknown as Parameters<typeof http.request>[0]);

        return response.data;
      },
      staleTime: ATTRIBUTE_STALE_TIME,
    })),
    combine: (results: UseQueryResult<ApiUserType>[]) => ({
      attributes: aggregateAttributes(results),
      isLoadingSchemas: results.some((result) => result.isLoading),
      schemaError: results.find((result) => result.error)?.error ?? null,
      failedSchemaRefetches: results.filter((result) => result.isError).map((result) => result.refetch),
    }),
  });

  const refetch = (): void => {
    if (listError) {
      refetchList().catch(() => undefined);
      return;
    }
    failedSchemaRefetches.forEach((refetchSchema) => {
      refetchSchema().catch(() => undefined);
    });
  };

  return {
    attributes,
    isLoading: isLoadingList || isLoadingSchemas,
    // A disabled list can still carry an earlier failure of the same shared query.
    error: enabled ? (listError ?? schemaError) : null,
    refetch,
  };
}
