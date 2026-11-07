// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowVariableDefinition} from '@thunderid/react';
import type {ReactFlowState} from '@xyflow/react';
import type {VariablesGroup} from '../context/FlowVariablesContext';
import {StaticStepTypes} from '../models/steps';

const NO_VARIABLES: FlowVariableDefinition[] = [];

/**
 * Reads the variables of every variables node from the React Flow store, by node, in the order the nodes are on the
 * canvas. A node that declares none holds a shared empty list, so what the groups hold compares by reference.
 *
 * @param state - The React Flow store state
 * @returns The variables of each variables node
 */
export function selectVariableGroups(state: ReactFlowState): VariablesGroup[] {
  return state.nodes
    .filter((node) => node.type === StaticStepTypes.Variables)
    .map((node) => {
      const inputs: unknown = (node.data as {inputs?: unknown} | undefined)?.inputs;

      return {nodeId: node.id, variables: Array.isArray(inputs) ? (inputs as FlowVariableDefinition[]) : NO_VARIABLES};
    });
}

/**
 * Tells whether two reads of the variable groups hold the same variables. A node moving on every drag tick builds a
 * new node object but keeps the list its variables are in, so comparing the lists by reference is enough.
 *
 * @param a - The groups read before
 * @param b - The groups read now
 * @returns Whether no node or variable list changed
 */
export function areVariableGroupsEqual(a: VariablesGroup[], b: VariablesGroup[]): boolean {
  return (
    a.length === b.length &&
    a.every((group, index) => group.nodeId === b[index].nodeId && group.variables === b[index].variables)
  );
}
