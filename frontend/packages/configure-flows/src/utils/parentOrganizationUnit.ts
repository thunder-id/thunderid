// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FlowType} from '../models/flows';
import type {FlowNode} from '../models/responses';
import {ExecutionTypes} from '../models/steps';

/** Parent OU a Resolve OU step carries until the organization unit its flow was created in is known. */
export const PARENT_OU_ID_PLACEHOLDER = '{{PARENT_OU_ID}}';

/**
 * Whether a parent OU is unset: missing, blank, or the placeholder.
 */
export const isParentOrganizationUnitUnset = (value: unknown): boolean =>
  typeof value !== 'string' || value.trim() === '' || value === PARENT_OU_ID_PLACEHOLDER;

/**
 * Sets the parent OU of every Resolve OU step that prompts in a sign-in flow and has none to `ouId`, the organization
 * unit the flow is created in. The server requires a parent there. Returns `nodes` itself when no step changes.
 */
export const fillParentOrganizationUnit = (nodes: FlowNode[], flowType: FlowType, ouId: string): FlowNode[] => {
  if (flowType !== FlowType.AUTHENTICATION) {
    return nodes;
  }

  let changed = false;
  const filled = nodes.map((node: FlowNode) => {
    if (
      node.executor?.name !== ExecutionTypes.OUResolverExecutor ||
      node.properties?.['resolveFrom'] !== 'prompt' ||
      !isParentOrganizationUnitUnset(node.properties['parentOuId'])
    ) {
      return node;
    }
    changed = true;

    return {...node, properties: {...node.properties, parentOuId: ouId}};
  });

  return changed ? filled : nodes;
};
