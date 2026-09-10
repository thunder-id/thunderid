// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AssertionConfig, OAuth2Config} from '@thunderid/configure-applications';
/**
 * Agent types are restricted to a single bootstrap-provisioned `default` schema. The constant
 * is shared by the create wizard (auto-pick the singleton) and the agent listing's Schema button.
 */
export const DEFAULT_AGENT_TYPE_NAME = 'default';

export type OAuthAgentConfig = OAuth2Config;

export interface AgentInboundAuthConfig {
  type: 'oauth2';
  config?: OAuthAgentConfig;
}

export interface AgentLoginConsentConfig {
  validityPeriod?: number;
}

export interface Agent {
  id: string;
  ouId: string;
  ouHandle?: string;
  type: string;
  name: string;
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
}

export interface BasicAgent {
  id: string;
  ouId: string;
  ouHandle?: string;
  type: string;
  name: string;
  description?: string;
  logoUrl?: string;
  clientId?: string;
  isReadOnly?: boolean;
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
  name: string;
  description?: string;
  logoUrl?: string;
  owner?: string;
  attributes?: Record<string, unknown>;
  inboundAuthConfig?: AgentInboundAuthConfig[];
}

export interface UpdateAgentRequest {
  ouId?: string;
  type?: string;
  name?: string;
  description?: string;
  logoUrl?: string;
  owner?: string;
  attributes?: Record<string, unknown>;
  allowedUserTypes?: string[];
  allowedAgentTypes?: string[];
  inboundAuthConfig?: AgentInboundAuthConfig[];
  authFlowId?: string;
  registrationFlowId?: string;
  isRegistrationFlowEnabled?: boolean;
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
