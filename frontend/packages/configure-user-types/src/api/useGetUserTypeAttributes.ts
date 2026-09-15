// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries, type UseQueryResult} from '@tanstack/react-query';
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
  /** Names of the user types declaring this attribute. */
  userTypes: string[];
}

/**
 * Result of {@link useGetUserTypeAttributes}.
 */
export interface UseGetUserTypeAttributesResult {
  attributes: AggregatedUserTypeAttribute[];
  isLoading: boolean;
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
        if (!existing.userTypes.includes(userType.name)) {
          existing.userTypes.push(userType.name);
        }
      } else {
        aggregated.set(key, {attribute, credential, userTypes: [userType.name]});
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
 * @returns The aggregated attributes and the combined loading state.
 */
export default function useGetUserTypeAttributes(): UseGetUserTypeAttributesResult {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const logger = useLogger('useGetUserTypeAttributes');

  const {data: userTypesData, isLoading: isLoadingList} = useGetUserTypes({limit: USER_TYPE_FETCH_LIMIT});

  const totalUserTypes: number = userTypesData?.totalResults ?? 0;
  if (totalUserTypes > USER_TYPE_FETCH_LIMIT) {
    logger.warn('Attribute suggestions cover only the first user types returned by the server', {
      fetched: USER_TYPE_FETCH_LIMIT,
      totalResults: totalUserTypes,
    });
  }

  const {attributes, isLoadingSchemas} = useQueries({
    queries: (userTypesData?.types ?? []).map((userType) => ({
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
    }),
  });

  return {attributes, isLoading: isLoadingList || isLoadingSchemas};
}
