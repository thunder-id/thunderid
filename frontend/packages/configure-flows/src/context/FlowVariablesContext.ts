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
   * How many steps hold a property bound to a variable.
   */
  countBindings: (identifier: string) => number;
  /**
   * Whether a sharing policy of the flow states a rule for a variable, which saving the flow without it deletes along
   * with the values units saved for it.
   */
  isShared: (identifier: string) => boolean;
  /**
   * Points every property bound to a variable at another identifier.
   */
  renameBindings: (from: string, to: string) => void;
  /**
   * Takes every property off a variable it is bound to, so the step falls back to its own field.
   */
  removeBindings: (identifier: string) => void;
  /**
   * The organization unit that owns the flow, which decides the connections a variable can name, or `undefined` while
   * the flow is not saved yet.
   */
  ownerOuId: string | undefined;
  /**
   * Opens the variables node on the canvas: selects it and shows its properties, where its variables are declared.
   */
  openVariablesNode: (nodeId: string) => void;
  /**
   * The id of the variables node nearest before a step, the flow having to run it first for the step to read what it
   * declares, or undefined when none comes before the step.
   */
  variablesNodeBefore: (stepId: string) => string | undefined;
  /**
   * The variables a step can use: the ones declared by the variables nodes every route passes before the step, which
   * is all the server accepts a step to refer to.
   */
  variablesBefore: (stepId: string) => FlowVariableDefinition[];
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
  countBindings: () => 0,
  groups: [],
  isShared: () => false,
  removeBindings: () => undefined,
  renameBindings: () => undefined,
  openVariablesNode: () => undefined,
  ownerOuId: undefined,
  variables: undefined,
  variablesBefore: () => [],
  variablesNodeBefore: () => undefined,
});

export default FlowVariablesContext;
