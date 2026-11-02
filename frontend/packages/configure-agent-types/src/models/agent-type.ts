// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AgentTypeDefinition} from './property-definition';

/**
 * System-level metadata for an agent type.
 */
export interface SystemAttributes {
  display?: string;
}

/**
 * Complete agent type object as returned by the API.
 */
export interface ApiAgentType {
  id: string;
  handle: string;
  displayName: string;
  ouId: string;
  ouHandle?: string;
  systemAttributes?: SystemAttributes;
  schema: AgentTypeDefinition;
}

/**
 * Agent type list item (minimal representation).
 */
export interface AgentTypeListItem {
  id: string;
  handle: string;
  displayName: string;
  ouId: string;
  ouHandle?: string;
  systemAttributes?: SystemAttributes;
}
