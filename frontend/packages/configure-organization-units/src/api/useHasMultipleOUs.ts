// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import {useCallback} from 'react';
import useGetChildOrganizationUnits from './useGetChildOrganizationUnits';
import useGetOrganizationUnits from './useGetOrganizationUnits';
import type {OrganizationUnit} from '../models/organization-unit';

interface UseHasMultipleOUsResult {
  hasMultipleOUs: boolean;
  isLoading: boolean;
  /** True when a lookup failed, so the answer in `hasMultipleOUs` and `ouList` may be incomplete. */
  isError: boolean;
  /** The failed lookup's error, or null while every lookup succeeds. */
  error: Error | null;
  /** Retries the lookups that failed. */
  refetch: () => void;
  ouList: OrganizationUnit[];
}

export default function useHasMultipleOUs(): UseHasMultipleOUsResult {
  // An admin confined to an organization unit creates resources only in that unit, so it is the one to offer.
  const {confinedOuId} = useCapabilitiesContext();
  const {
    data: ouData,
    isLoading: isOuLoading,
    isError: isOuError,
    error: ouError,
    refetch: refetchOus,
  } = useGetOrganizationUnits({limit: 2, offset: 0});
  const ouList = ouData?.organizationUnits ?? [];
  const rootCount = ouData?.totalResults ?? 0;
  const singleRootId = rootCount === 1 && !confinedOuId ? ouList[0]?.id : undefined;

  const {
    data: childData,
    isLoading: isChildLoading,
    isError: isChildError,
    error: childError,
    refetch: refetchChildren,
  } = useGetChildOrganizationUnits(singleRootId, {
    limit: 1,
    offset: 0,
  });

  const hasMultipleRoots = rootCount > 1;
  const singleRootHasChildren = Boolean(singleRootId) && (childData?.totalResults ?? 0) > 0;
  const isChildLookupFailed = Boolean(singleRootId) && isChildError;
  const refetch = useCallback((): void => {
    if (isOuError) void refetchOus();
    if (isChildLookupFailed) void refetchChildren();
  }, [isOuError, isChildLookupFailed, refetchOus, refetchChildren]);

  return {
    hasMultipleOUs: hasMultipleRoots || singleRootHasChildren,
    isLoading: isOuLoading || (Boolean(singleRootId) && isChildLoading),
    isError: isOuError || isChildLookupFailed,
    error: (isOuError ? ouError : null) ?? (isChildLookupFailed ? childError : null),
    refetch,
    ouList,
  };
}
