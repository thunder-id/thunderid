// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {PermissionDelimiter} from './permissions';

export type ResourceServerType = 'API' | 'MCP' | 'CUSTOM';
export const AuthorizationEngines = {
  RBAC: 'rbac',
  AUTHZEN_PDP: 'authzen_pdp',
} as const;
export type AuthorizationEngine = (typeof AuthorizationEngines)[keyof typeof AuthorizationEngines];

export interface AuthorizationEngineConfig {
  type: AuthorizationEngine;
  properties?: {
    pdpConnectionId?: string;
  };
}

export interface AuthZENPDPConnectionSummary {
  id: string;
  name: string;
  description?: string;
}

export interface AuthZENPDPConnectionListResponse {
  totalResults: number;
  connections: AuthZENPDPConnectionSummary[];
}

const DEFAULT_ELIGIBLE_TYPES: readonly ResourceServerType[] = ['API', 'CUSTOM'];

// MCP servers are always addressed by their own identifier, so they never act as the fallback.
export function isDefaultEligibleType(type: ResourceServerType | undefined): boolean {
  return type !== undefined && DEFAULT_ELIGIBLE_TYPES.includes(type);
}

export interface ResourceServer {
  id: string;
  name: string;
  description?: string | null;
  identifier: string;
  ouId: string;
  delimiter: string;
  isReadOnly?: boolean;
  type: ResourceServerType;
  authorizationEngine?: AuthorizationEngineConfig;
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

export type ActionKind = 'tool' | 'resource';

export interface Action {
  id: string;
  name: string;
  handle: string;
  description?: string | null;
  permission: string;
  kind?: ActionKind;
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
  description?: string;
  identifier: string;
  delimiter?: PermissionDelimiter;
  ouId: string;
  type?: ResourceServerType;
}

export interface UpdateResourceServerRequest {
  name: string;
  description?: string | null;
  identifier: string;
  ouId: string;
  authorizationEngine?: AuthorizationEngineConfig;
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
  kind?: ActionKind;
}

export interface UpdateActionRequest {
  name?: string;
  description?: string | null;
}

export interface ResourcePermissions {
  resourceServerId: string;
  permissions: string[];
}

export interface DefaultResourceServerValue {
  resourceServerId?: string;
}

export interface ServerConfigLayers<T> {
  readOnly: T;
  writable: T;
  merged: T;
}

export type DefaultResourceServerConfigResponse = ServerConfigLayers<DefaultResourceServerValue>;
