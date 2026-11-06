// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import useRequest from './request';

/** One of the console's administrators, as the users list gives them. */
export interface Administrator {
  id: string;
  display?: string;
  attributes?: Record<string, unknown>;
}

/** The first administrators and how many there are. */
export interface Administrators {
  totalResults: number;
  users: Administrator[];
}

/** The first few of the console's administrators, the users its Administrators page lists, and their count. */
export default function useAdministrators(limit: number): UseQueryResult<Administrators> {
  const request = useRequest();
  return useQuery<Administrators>({
    queryKey: ['environment-administrators', limit],
    queryFn: () => request<Administrators>('GET', `/users?limit=${limit}&offset=0&include=display`),
    retry: false,
    staleTime: 60_000,
  });
}
