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
import {describe, expect, it} from 'vitest';
import {stripSimulationEdgeClasses, stripSimulationNodeClasses, withSimulationClasses} from '../stripSimulationClasses';

const makeNode = (overrides: Partial<Node>): Node =>
  ({id: 'node-1', position: {x: 0, y: 0}, data: {}, ...overrides}) as Node;

const makeEdge = (overrides: Partial<Edge>): Edge => ({id: 'edge-1', source: 'a', target: 'b', ...overrides}) as Edge;

describe('stripSimulationNodeClasses', () => {
  it('should return the same array when no node carries simulation classes', () => {
    const nodes = [makeNode({className: 'my-class'}), makeNode({id: 'node-2'})];

    expect(stripSimulationNodeClasses(nodes)).toBe(nodes);
  });

  it('should remove simulation classes from affected nodes', () => {
    const nodes = [makeNode({className: 'simulation-dimmed'})];

    expect(stripSimulationNodeClasses(nodes)[0].className).toBeUndefined();
  });

  it('should preserve unrelated classes when stripping', () => {
    const nodes = [makeNode({className: 'my-class simulation-preview-target simulation-kind-success'})];

    expect(stripSimulationNodeClasses(nodes)[0].className).toBe('my-class');
  });

  it('should keep untouched nodes referentially identical', () => {
    const clean = makeNode({id: 'node-2', className: 'my-class'});
    const nodes = [makeNode({className: 'simulation-path'}), clean];

    expect(stripSimulationNodeClasses(nodes)[1]).toBe(clean);
  });
});

describe('withSimulationClasses', () => {
  it('should return the simulation classes when there is no existing class', () => {
    expect(withSimulationClasses(undefined, 'simulation-dimmed')).toBe('simulation-dimmed');
  });

  it('should preserve existing classes when decorating', () => {
    expect(withSimulationClasses('my-class', 'simulation-path')).toBe('my-class simulation-path');
  });

  it('should replace previous simulation decoration instead of stacking it', () => {
    expect(withSimulationClasses('my-class simulation-dimmed', 'simulation-path')).toBe('my-class simulation-path');
  });
});

describe('stripSimulationEdgeClasses', () => {
  it('should return the same array when no edge carries simulation classes', () => {
    const edges = [makeEdge({animated: true, className: 'my-edge'})];

    expect(stripSimulationEdgeClasses(edges)).toBe(edges);
  });

  it('should remove simulation classes and reset the traversal animation', () => {
    const edges = [makeEdge({animated: true, className: 'simulation-path simulation-kind-action'})];

    const [stripped] = stripSimulationEdgeClasses(edges);

    expect(stripped.className).toBeUndefined();
    expect(stripped.animated).toBe(false);
  });

  it('should not reset animation on edges without simulation classes', () => {
    const animatedEdge = makeEdge({id: 'edge-2', animated: true});
    const edges = [makeEdge({className: 'simulation-dimmed'}), animatedEdge];

    expect(stripSimulationEdgeClasses(edges)[1]).toBe(animatedEdge);
  });
});
