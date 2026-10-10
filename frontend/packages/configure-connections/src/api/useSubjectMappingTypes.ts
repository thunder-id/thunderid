// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys, {SUBJECT_MAPPING_TYPES_PAGE_SIZE} from '../constants/query-keys';
import type {SubjectMappingTypeLists} from '../models/subject-mapping';

interface SubjectTypeListResponse {
  totalResults: number;
  types: {id: string; handle: string; displayName: string}[];
}

export default function useSubjectMappingTypes(): UseQueryResult<SubjectMappingTypeLists> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<SubjectMappingTypeLists>({
    queryKey: [ConnectionQueryKeys.SUBJECT_MAPPING_TYPES],
    queryFn: async (): Promise<SubjectMappingTypeLists> => {
      const serverUrl = getServerUrl();
      const fetchPage = async (
        path: 'agent-types' | 'user-types',
        offset: number,
      ): Promise<SubjectTypeListResponse> => {
        const response: {data: SubjectTypeListResponse} = await http.request({
          url: `${serverUrl}/${path}?limit=${SUBJECT_MAPPING_TYPES_PAGE_SIZE}&offset=${offset}&include=display`,
          method: 'GET',
          headers: {'Content-Type': 'application/json'},
        } as unknown as Parameters<typeof http.request>[0]);

        return response.data;
      };
      const fetchAllPages = async (
        path: 'agent-types' | 'user-types',
      ): Promise<{id: string; handle: string; displayName: string}[]> => {
        const firstPage = await fetchPage(path, 0);
        const remainingOffsets = Array.from(
          {length: Math.max(0, Math.ceil(firstPage.totalResults / SUBJECT_MAPPING_TYPES_PAGE_SIZE) - 1)},
          (_, index) => (index + 1) * SUBJECT_MAPPING_TYPES_PAGE_SIZE,
        );
        const remainingPages = await Promise.all(remainingOffsets.map((offset) => fetchPage(path, offset)));

        return [firstPage, ...remainingPages].flatMap((page) => page.types);
      };
      const [userTypes, agentTypes] = await Promise.all([fetchAllPages('user-types'), fetchAllPages('agent-types')]);

      return {
        userTypes: userTypes.map((type) => ({...type, category: 'user'})),
        agentTypes: agentTypes.map((type) => ({...type, category: 'agent'})),
      };
    },
  });
}
