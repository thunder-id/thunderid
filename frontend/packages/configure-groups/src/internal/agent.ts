// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// TEMPORARY: mirrors the two response shapes AddMemberDialog needs from
// apps/console/src/features/agents/models/agent.ts. The agents feature hasn't been extracted
// into @thunderid/configure-agents yet; once it is, this file should be deleted and
// AddMemberDialog should import from that package instead.

/** The kind of a personal agent, which acts only for its owner and cannot hold roles or groups. */
export const PERSONAL_AGENT_KIND = 'personal-assistant';

export interface BasicAgent {
  id: string;
  ouId: string;
  ouHandle?: string;
  type: string;
  description?: string;
  logoUrl?: string;
  clientId?: string;
  display?: string;
  isReadOnly?: boolean;
  kind?: string;
}

export interface AgentListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  agents: BasicAgent[];
}
