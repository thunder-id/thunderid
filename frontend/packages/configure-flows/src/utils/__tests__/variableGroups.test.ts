// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowVariableDefinition} from '@thunderid/react';
import type {ReactFlowState} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import {areVariableGroupsEqual, selectVariableGroups} from '../variableGroups';

const makeState = (nodes: unknown[]): ReactFlowState => ({nodes}) as unknown as ReactFlowState;
const variable = (identifier: string): FlowVariableDefinition => ({defaultValue: '', identifier, type: 'TEXT_INPUT'});

describe('selectVariableGroups', () => {
  it('should read the variables of each variables node, in canvas order, and skip other nodes', () => {
    const a = [variable('a')];
    const b = [variable('b')];

    expect(
      selectVariableGroups(
        makeState([
          {id: 'v-1', type: 'VARIABLES', data: {inputs: a}},
          {id: 'step', type: 'VIEW', data: {inputs: [variable('x')]}},
          {id: 'v-2', type: 'VARIABLES', data: {inputs: b}},
        ]),
      ),
    ).toEqual([
      {nodeId: 'v-1', variables: a},
      {nodeId: 'v-2', variables: b},
    ]);
  });

  it('should hold no variables for a node that declares none, as one shared list', () => {
    const [first, second] = selectVariableGroups(
      makeState([
        {id: 'v-1', type: 'VARIABLES', data: {}},
        {id: 'v-2', type: 'VARIABLES', data: {inputs: 'not a list'}},
      ]),
    );

    expect(first.variables).toEqual([]);
    expect(first.variables).toBe(second.variables);
  });

  it('should find nothing in a flow without a variables node', () => {
    expect(selectVariableGroups(makeState([{id: 'step', type: 'VIEW', data: {}}]))).toEqual([]);
  });
});

describe('areVariableGroupsEqual', () => {
  const inputs = [variable('a')];

  it('should treat a moved node as unchanged, since its variable list is the same one', () => {
    const before = selectVariableGroups(
      makeState([{id: 'v-1', type: 'VARIABLES', position: {x: 0, y: 0}, data: {inputs}}]),
    );
    const after = selectVariableGroups(
      makeState([{id: 'v-1', type: 'VARIABLES', position: {x: 40, y: 0}, data: {inputs}}]),
    );

    expect(areVariableGroupsEqual(before, after)).toBe(true);
  });

  it('should see a changed variable list, a node added, or a node removed', () => {
    const base = [{nodeId: 'v-1', variables: inputs}];

    expect(areVariableGroupsEqual(base, [{nodeId: 'v-1', variables: [variable('a')]}])).toBe(false);
    expect(areVariableGroupsEqual(base, [...base, {nodeId: 'v-2', variables: inputs}])).toBe(false);
    expect(areVariableGroupsEqual(base, [])).toBe(false);
  });
});
