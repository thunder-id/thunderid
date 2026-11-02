// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import type {Gateway} from '../models/gateway';

/** The targets configuration runs on, the default one first: a control plane's gateways by default. */
export default function useGateways(): UseQueryResult<Gateway[]> {
  const source = useDeploySource();
  return useQuery<Gateway[]>({
    queryKey: [DeployQueryKeys.GATEWAYS],
    queryFn: (): Promise<Gateway[]> => source.targets(),
    staleTime: 60_000,
  });
}
