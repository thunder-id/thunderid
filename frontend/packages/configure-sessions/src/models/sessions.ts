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

export interface SessionParticipant {
  appId: string;
  /** Server-resolved application name; absent when unresolved, so the UI falls back to appId. */
  appName?: string;
  firstJoinedAt: string;
  lastActiveAt: string;
}

export interface Session {
  id: string;
  userId: string;
  /** Server-resolved user display name; absent when unresolved, so the UI falls back to userId. */
  userName?: string;
  loginFlowId: string;
  authenticatedAt: string;
  createdAt: string;
  lastActiveAt: string;
  idleExpiresAt?: string;
  absoluteExpiresAt?: string;
  participants: SessionParticipant[];
}

export interface SessionListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  sessions: Session[];
  links: {href: string; rel: string}[];
}

export type SessionListFilter = {userId: string; appId?: never} | {appId: string; userId?: never};
