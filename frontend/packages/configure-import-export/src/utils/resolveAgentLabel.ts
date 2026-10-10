// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export interface AgentLike {
  id?: string;
  type?: string;
  attributes?: Record<string, unknown>;
}

export interface AgentTypeLike {
  handle?: string;
  systemAttributes?: {display?: string};
}

const readDisplayValue = (attributes: Record<string, unknown> | undefined, path: string): string | undefined => {
  let current: unknown = attributes;

  for (const part of path.split('.')) {
    if (typeof current !== 'object' || current === null || Array.isArray(current)) return undefined;
    current = (current as Record<string, unknown>)[part];
  }

  if (typeof current === 'string') return current || undefined;
  if (typeof current === 'number' && Number.isFinite(current)) return String(current);

  return undefined;
};

const indexByHandle = (types: AgentTypeLike[]): Map<string, AgentTypeLike> =>
  new Map(types.flatMap((type) => (type.handle ? [[type.handle, type] as const] : [])));

export const getMissingAgentTypeHandles = (agents: AgentLike[], localTypes: AgentTypeLike[]): string[] => {
  const local = indexByHandle(localTypes);

  return [...new Set(agents.flatMap((agent) => (agent.type && !local.has(agent.type) ? [agent.type] : [])))];
};

export const createAgentLabelResolver = (
  localTypes: AgentTypeLike[],
  serverTypes: AgentTypeLike[],
): ((agent: AgentLike) => string | undefined) => {
  const local = indexByHandle(localTypes);
  const server = indexByHandle(serverTypes);

  return (agent) => {
    const type = agent.type ? (local.get(agent.type) ?? server.get(agent.type)) : undefined;
    const path = type?.systemAttributes?.display;
    const value = path ? readDisplayValue(agent.attributes, path) : undefined;

    return value ?? agent.id;
  };
};
