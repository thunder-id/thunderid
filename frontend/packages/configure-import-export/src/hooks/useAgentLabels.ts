// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetAgentTypes} from '@thunderid/configure-agent-types';
import {useMemo} from 'react';
import {
  createAgentLabelResolver,
  getMissingAgentTypeHandles,
  type AgentLike,
  type AgentTypeLike,
} from '../utils/resolveAgentLabel';

const asArray = <T>(value: unknown): T[] => (Array.isArray(value) ? (value as T[]) : []);

export default function useAgentLabels(
  configData: Record<string, unknown> | null | undefined,
): (agent: AgentLike) => string | undefined {
  const agents = useMemo(() => asArray<AgentLike>(configData?.['agent']), [configData]);
  const localTypes = useMemo(() => asArray<AgentTypeLike>(configData?.['agent_type']), [configData]);
  const hasMissingTypes = useMemo(
    () => getMissingAgentTypeHandles(agents, localTypes).length > 0,
    [agents, localTypes],
  );

  const {data} = useGetAgentTypes(undefined, {enabled: hasMissingTypes});
  const serverTypes = data?.types;

  return useMemo(() => createAgentLabelResolver(localTypes, serverTypes ?? []), [localTypes, serverTypes]);
}
