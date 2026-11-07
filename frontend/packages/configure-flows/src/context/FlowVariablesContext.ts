// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowVariableDefinition} from '@thunderid/react';
import {createContext} from 'react';

/**
 * The variables one variables node of the flow declares.
 */
export interface VariablesGroup {
  /**
   * The id of the variables node.
   */
  nodeId: string;
  variables: FlowVariableDefinition[];
}

/**
 * The variables the flow on the canvas declares, grouped by the variables node that declares them, and a way to
 * change them.
 */
export interface FlowVariablesContextValue {
  /**
   * The variables of each variables node of the flow, in the order the nodes are on the canvas. Empty while the flow
   * has no variables node.
   */
  groups: VariablesGroup[];
  /**
   * Replaces the variables of the nodes the groups name. Does nothing for a node that is no longer on the canvas.
   */
  setGroups: (groups: VariablesGroup[]) => void;
  /**
   * Every variable of the flow, whichever node declares it, or undefined while the flow has no variables node.
   */
  variables: FlowVariableDefinition[] | undefined;
}

/**
 * Lets a property of a step name a variable. Outside a host that provides it (e.g. isolated renders in tests),
 * the flow has no variables node.
 */
const FlowVariablesContext = createContext<FlowVariablesContextValue>({
  groups: [],
  setGroups: () => undefined,
  variables: undefined,
});

export default FlowVariablesContext;
