// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Allowed resource types for product configuration import/export
 */
export const ALLOWED_RESOURCE_TYPES = [
  'application',
  'group',
  'user',
  'flow',
  'theme',
  'layout',
  'translation',
  'connection',
  'organization_unit',
  'project',
  'user_type',
  'agent_type',
  'resource_server',
  'role',
  'agent',
  'presentation_definition',
  'credential_configuration',
  'server_config',
] as const;

export type ResourceType = (typeof ALLOWED_RESOURCE_TYPES)[number];
