// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';

/** Reads how many of a listed resource there are, from a list of one. */
export function useCountReader(): (path: string) => Promise<number> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  return async (path: string): Promise<number> => {
    const separator = path.includes('?') ? '&' : '?';
    const response = (await http.request({
      url: `${getServerUrl()}${path}${separator}limit=1&offset=0`,
      method: 'GET',
    } as unknown as Parameters<typeof http.request>[0])) as {data: {totalResults?: number}};
    return response.data.totalResults ?? 0;
  };
}

/** How many of a listed resource there are, once a path is known. */
function useCount(path: string | undefined): number | undefined {
  const read = useCountReader();
  return useQuery<number>({
    queryKey: ['organization-home-count', path],
    queryFn: () => read(path ?? ''),
    enabled: path !== undefined,
    retry: false,
    staleTime: 60_000,
  }).data;
}

export const applicationsPath = (projectId: string): string =>
  `/applications?projectId=${encodeURIComponent(projectId)}`;
export const agentsPath = (projectId: string): string => `/agents?projectId=${encodeURIComponent(projectId)}`;

/** How many applications and agents a project holds, as a person reads it; empty until they are read. */
export default function useProjectCounts(projectId: string | undefined): string {
  const {t} = useTranslation();
  const applications = useCount(projectId ? applicationsPath(projectId) : undefined);
  const agents = useCount(projectId ? agentsPath(projectId) : undefined);
  const countOf = (count: number | undefined, key: string, one: string, other: string, none: string): string => {
    if (count === undefined) return '';
    if (count === 0) return t(`home:organization.${key}None`, none);
    return t(`home:organization.${key}`, {count, defaultValue_one: one, defaultValue_other: other});
  };
  return [
    countOf(applications, 'applications', '{{count}} application', '{{count}} applications', 'No applications'),
    countOf(agents, 'agents', '{{count}} agent', '{{count}} agents', 'No agents'),
  ]
    .filter(Boolean)
    .join(' · ');
}
