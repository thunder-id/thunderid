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

import type {PermissionDelimiter} from './permissions';

export type ResourceServerType = 'API' | 'MCP' | 'CUSTOM';

export interface ResourceServer {
  id: string;
  name: string;
  description?: string | null;
  handle: string;
  identifier?: string | null;
  ouId: string;
  delimiter: string;
  isReadOnly?: boolean;
  type: ResourceServerType;
}

export interface ResourceServerListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  resourceServers: ResourceServer[];
  links?: {rel: string; href: string}[];
}

export interface Resource {
  id: string;
  name: string;
  handle: string;
  description?: string | null;
  parent?: string | null;
  permission: string;
}

export interface ResourceListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  resources: Resource[];
  links?: {rel: string; href: string}[];
}

export interface Action {
  id: string;
  name: string;
  handle: string;
  description?: string | null;
  permission: string;
}

export interface ActionListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  actions: Action[];
  links?: {rel: string; href: string}[];
}

export interface CreateResourceServerRequest {
  name: string;
  handle: string | null;
  description?: string;
  identifier?: string;
  delimiter?: PermissionDelimiter;
  ouId: string;
  type?: ResourceServerType;
}

export interface UpdateResourceServerRequest {
  name?: string;
  description?: string | null;
  identifier?: string | null;
}

export interface CreateResourceRequest {
  name: string;
  handle: string;
  description?: string;
  parent?: string;
}

export interface UpdateResourceRequest {
  name?: string;
  description?: string | null;
}

export interface CreateActionRequest {
  name: string;
  handle: string;
  description?: string;
}

export interface UpdateActionRequest {
  name?: string;
  description?: string | null;
}
