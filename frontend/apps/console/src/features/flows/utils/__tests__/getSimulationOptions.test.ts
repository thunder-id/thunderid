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

import type {Edge, Node} from '@xyflow/react';
import {describe, it, expect} from 'vitest';
import getSimulationOptions, {SimulationOptionKinds} from '../getSimulationOptions';

const createNode = (id: string, data: Record<string, unknown> = {}): Node =>
  ({id, type: 'VIEW', position: {x: 0, y: 0}, data}) as Node;

const createEdge = (id: string, source: string, sourceHandle: string, target: string): Edge =>
  ({id, source, sourceHandle, target}) as Edge;

describe('getSimulationOptions', () => {
  const viewNode = createNode('view-1', {
    components: [
      {
        id: 'block-1',
        type: 'BLOCK',
        components: [{id: 'signin-button', type: 'ACTION', label: 'Sign In'}],
      },
      {
        id: 'google-block',
        type: 'BLOCK',
        components: [{id: 'google-button', type: 'ACTION', label: 'Continue with Google'}],
      },
    ],
  });
  const executorNode = createNode('executor-1');
  const failureNode = createNode('failure-view');
  const endNode = createNode('end-1');

  const nodes = [viewNode, executorNode, failureNode, endNode];

  it('should label view button transitions with the button text', () => {
    const edges = [
      createEdge('e1', 'view-1', 'signin-button_NEXT', 'executor-1'),
      createEdge('e2', 'view-1', 'google-button_NEXT', 'executor-1'),
    ];

    const options = getSimulationOptions('view-1', nodes, edges);

    expect(options).toHaveLength(2);
    expect(options[0]).toMatchObject({kind: SimulationOptionKinds.Action, actionLabel: 'Sign In'});
    expect(options[1]).toMatchObject({kind: SimulationOptionKinds.Action, actionLabel: 'Continue with Google'});
  });

  it('should classify step-level transitions as success', () => {
    const edges = [createEdge('e1', 'executor-1', 'executor-1_NEXT', 'end-1')];

    const options = getSimulationOptions('executor-1', nodes, edges);

    expect(options).toEqual([{edgeId: 'e1', targetNodeId: 'end-1', kind: SimulationOptionKinds.Success}]);
  });

  it('should classify failure and incomplete transitions', () => {
    const edges = [
      createEdge('e1', 'executor-1', 'failure', 'failure-view'),
      createEdge('e2', 'executor-1', 'executor-1_INCOMPLETE', 'view-1'),
      createEdge('e3', 'executor-1', 'executor-1_NEXT', 'end-1'),
    ];

    const options = getSimulationOptions('executor-1', nodes, edges);

    expect(options.map((option) => option.kind)).toEqual([
      SimulationOptionKinds.Success,
      SimulationOptionKinds.Incomplete,
      SimulationOptionKinds.Failure,
    ]);
  });

  it('should ignore edges pointing to missing nodes and other sources', () => {
    const edges = [
      createEdge('e1', 'executor-1', 'executor-1_NEXT', 'nonexistent'),
      createEdge('e2', 'view-1', 'view-1_NEXT', 'end-1'),
    ];

    expect(getSimulationOptions('executor-1', nodes, edges)).toHaveLength(0);
  });

  it('should return an empty list for terminal nodes', () => {
    expect(getSimulationOptions('end-1', nodes, [])).toHaveLength(0);
  });
});
