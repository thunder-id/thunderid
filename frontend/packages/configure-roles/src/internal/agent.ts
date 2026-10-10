// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// TEMPORARY: mirrors the two response shapes AddAssignmentDialog needs from
// apps/console/src/features/agents/models/agent.ts. The agents feature hasn't been extracted
// into @thunderid/configure-agents yet; once it is, this file should be deleted and
// AddAssignmentDialog should import from that package instead.

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
}

export interface AgentListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  agents: BasicAgent[];
}
