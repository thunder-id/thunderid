// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SystemAttributes} from './agent-type';
import type {AgentTypeDefinition} from './property-definition';

/**
 * Request body for `POST /agent-types` (create).
 */
export interface CreateAgentTypeRequest {
  handle: string;
  displayName: string;
  ouId: string;
  systemAttributes?: SystemAttributes;
  schema: AgentTypeDefinition;
}

/**
 * Request body for `PUT /agent-types/{id}` (update).
 */
export interface UpdateAgentTypeRequest {
  displayName: string;
  ouId: string;
  systemAttributes?: SystemAttributes;
  schema: AgentTypeDefinition;
}
