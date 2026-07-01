/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {OAuth2Config} from '../../applications/models/oauth';
import type {AssertionConfig} from '../../applications/models/token';

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
  owner?: string;
  clientId?: string;
  attributes?: Record<string, unknown>;
  allowedUserTypes?: string[];
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
  owner?: string;
  attributes?: Record<string, unknown>;
  inboundAuthConfig?: AgentInboundAuthConfig[];
}

export interface UpdateAgentRequest {
  ouId?: string;
  type?: string;
  name?: string;
  description?: string;
  attributes?: Record<string, unknown>;
  allowedUserTypes?: string[];
  inboundAuthConfig?: AgentInboundAuthConfig[];
}
