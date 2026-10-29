// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AssertionConfig, OAuth2Config} from '@thunderid/configure-applications';
import type {AccessStatusReport} from '@thunderid/types';

/**
 * Handle of the bootstrap-provisioned `default` agent type, which the agent listing's Schema button
 * looks up to show the schema behind the listed agents.
 */
export const DEFAULT_AGENT_TYPE_HANDLE = 'default';

export type OAuthAgentConfig = OAuth2Config;

export interface AgentInboundAuthConfig {
  type: 'oauth2';
  config?: OAuthAgentConfig;
}

export interface AgentLoginConsentConfig {
  validityPeriod?: number;
}

/**
 * Where an agent stands in its lifespan, derived by the server on every read.
 */
export type AgentLifespanPhase = 'UNBOUNDED' | 'ACTIVE' | 'EXPIRED';

export interface AgentLifespan {
  phase: AgentLifespanPhase;
}

export interface Agent {
  id: string;
  ouId: string;
  ouHandle?: string;
  type: string;
  display?: string;
  description?: string;
  logoUrl?: string;
  owner?: string;
  clientId?: string;
  attributes?: Record<string, unknown>;
  allowedUserTypes?: string[];
  allowedAgentTypes?: string[];
  inboundAuthConfig?: AgentInboundAuthConfig[];
  // Inbound-client fields the agent shares with applications. Populated only when an inbound
  // client row exists for the agent (i.e., create modes 2 or 3 — not entity-only).
  authFlowId?: string;
  registrationFlowId?: string;
  isRegistrationFlowEnabled?: boolean;
  assertion?: AssertionConfig;
  loginConsent?: AgentLoginConsentConfig;
  isReadOnly?: boolean;
  /**
   * Effective access status, derived by the server at read time. Read-only.
   */
  status?: AccessStatusReport;
  /**
   * The RFC 3339 instant the agent's lifespan ends. Absent when it does not expire. On an edit, null
   * removes the end.
   */
  validUntil?: string | null;
  /**
   * Where the agent stands in its lifespan. Read-only.
   */
  lifespan?: AgentLifespan;
}

export interface BasicAgent {
  id: string;
  ouId: string;
  ouHandle?: string;
  type: string;
  display?: string;
  description?: string;
  logoUrl?: string;
  clientId?: string;
  attributes?: Record<string, unknown>;
  isReadOnly?: boolean;
  /**
   * Effective access status, derived by the server at read time. Read-only.
   */
  status?: AccessStatusReport;
  /**
   * The RFC 3339 instant the agent's lifespan ends. Absent when it does not expire. On an edit, null
   * removes the end.
   */
  validUntil?: string | null;
  /**
   * Where the agent stands in its lifespan. Read-only.
   */
  lifespan?: AgentLifespan;
}

export interface AgentListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  agents: BasicAgent[];
}

export interface CreateAgentRequest {
  ouId: string;
  type: string;
  description?: string;
  logoUrl?: string;
  owner?: string;
  attributes?: Record<string, unknown>;
  inboundAuthConfig?: AgentInboundAuthConfig[];
}

export interface UpdateAgentRequest {
  ouId?: string;
  type?: string;
  description?: string;
  logoUrl?: string;
  owner?: string;
  /** Whether the agent acts on behalf of a signed-in user. Not stored; it selects the grant rules checked on save. */
  delegated?: boolean;
  attributes?: Record<string, unknown>;
  allowedUserTypes?: string[];
  allowedAgentTypes?: string[];
  inboundAuthConfig?: AgentInboundAuthConfig[];
  authFlowId?: string;
  registrationFlowId?: string;
  isRegistrationFlowEnabled?: boolean;
  /** Absent leaves the lifespan unchanged; null removes its end. */
  validUntil?: string | null;
}

export interface AgentGroup {
  id: string;
  name: string;
  ouId: string;
}

export interface AgentGroupListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  groups: AgentGroup[];
}

export interface AgentRoleListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  roles: string[];
}
