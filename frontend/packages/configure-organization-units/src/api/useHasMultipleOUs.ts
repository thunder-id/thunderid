// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useGetChildOrganizationUnits from './useGetChildOrganizationUnits';
import useGetOrganizationUnits from './useGetOrganizationUnits';
import type {OrganizationUnit} from '../models/organization-unit';

interface UseHasMultipleOUsResult {
  hasMultipleOUs: boolean;
  isLoading: boolean;
  /** True when a lookup failed, so the answer in `hasMultipleOUs` and `ouList` may be incomplete. */
  isError: boolean;
  ouList: OrganizationUnit[];
}

export default function useHasMultipleOUs(): UseHasMultipleOUsResult {
  const {data: ouData, isLoading: isOuLoading, isError: isOuError} = useGetOrganizationUnits({limit: 2, offset: 0});
  const ouList = ouData?.organizationUnits ?? [];
  const rootCount = ouData?.totalResults ?? 0;
  const singleRootId = rootCount === 1 ? ouList[0]?.id : undefined;

  const {
    data: childData,
    isLoading: isChildLoading,
    isError: isChildError,
  } = useGetChildOrganizationUnits(singleRootId, {
    limit: 1,
    offset: 0,
  });

  const hasMultipleRoots = rootCount > 1;
  const singleRootHasChildren = rootCount === 1 && (childData?.totalResults ?? 0) > 0;

  return {
    hasMultipleOUs: hasMultipleRoots || singleRootHasChildren,
    isLoading: isOuLoading || (rootCount === 1 && isChildLoading),
    isError: isOuError || (rootCount === 1 && isChildError),
    ouList,
  };
}
