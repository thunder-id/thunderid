// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import collapseExecutorChains, {
  EXECUTION_STACK_NODE_TYPE,
  getExecutionStackWidth,
  type ExecutionStackData,
} from '../compactGraphTransforms';

const view = (id: string, x = 0): Node => ({data: {}, id, position: {x, y: 0}, type: 'VIEW'});
const executor = (id: string, x = 0): Node => ({
  data: {action: {executor: {name: `${id}-executor`}}, display: {label: `${id} label`}},
  id,
  position: {x, y: 0},
  type: 'TASK_EXECUTION',
});
const successEdge = (id: string, source: string, target: string): Edge => ({
  id,
  source,
  sourceHandle: `${source}_NEXT`,
  target,
});

describe('collapseExecutorChains', () => {
  it('should collapse a run of two executors into a stack with rewired edges', () => {
    const nodes = [view('view-1'), executor('exec-a', 100), executor('exec-b', 200), view('view-2', 300)];
    const edges = [
      successEdge('e1', 'view-1', 'exec-a'),
      successEdge('e2', 'exec-a', 'exec-b'),
      successEdge('e3', 'exec-b', 'view-2'),
    ];

    const result = collapseExecutorChains(nodes, edges);

    const stack = result.nodes.find((node) => node.type === EXECUTION_STACK_NODE_TYPE);
    expect(stack).toBeDefined();
    expect((stack?.data as ExecutionStackData).memberIds).toEqual(['exec-a', 'exec-b']);
    // Stack replaces the head at its position; members are gone
    expect(stack?.position).toEqual({x: 100, y: 0});
    expect(result.nodes.map((node) => node.id)).toEqual(['view-1', stack?.id, 'view-2']);

    // Interior edge dropped; boundary edges rewired but keep their ids
    expect(result.edges.map((edge) => edge.id)).toEqual(['e1', 'e3']);
    const incoming = result.edges.find((edge) => edge.id === 'e1');
    expect(incoming?.target).toBe(stack?.id);
    const outgoing = result.edges.find((edge) => edge.id === 'e3');
    expect(outgoing?.source).toBe(stack?.id);
    expect(outgoing?.sourceHandle).toBe(`${stack?.id}_NEXT`);
    expect(outgoing?.target).toBe('view-2');

    expect(result.stackMembersById.get(stack?.id ?? '')).toEqual(['exec-a', 'exec-b']);
  });

  it('should make the stack non-connectable and non-deletable', () => {
    const nodes = [view('view-1'), executor('exec-a'), executor('exec-b'), view('view-2')];
    const edges = [
      successEdge('e1', 'view-1', 'exec-a'),
      successEdge('e2', 'exec-a', 'exec-b'),
      successEdge('e3', 'exec-b', 'view-2'),
    ];

    const result = collapseExecutorChains(nodes, edges);

    // A stack id has no counterpart in the real graph, so connecting to or
    // deleting one would leave edges pointing at a node that does not exist.
    const stack = result.nodes.find((node) => node.type === EXECUTION_STACK_NODE_TYPE);
    expect(stack?.connectable).toBe(false);
    expect(stack?.deletable).toBe(false);
  });

  it('should collapse chains longer than two members', () => {
    const nodes = [view('view-1'), executor('a'), executor('b'), executor('c'), view('view-2')];
    const edges = [
      successEdge('e1', 'view-1', 'a'),
      successEdge('e2', 'a', 'b'),
      successEdge('e3', 'b', 'c'),
      successEdge('e4', 'c', 'view-2'),
    ];

    const result = collapseExecutorChains(nodes, edges);

    const stack = result.nodes.find((node) => node.type === EXECUTION_STACK_NODE_TYPE);
    expect((stack?.data as ExecutionStackData).memberIds).toEqual(['a', 'b', 'c']);
    expect(result.edges).toHaveLength(2);
  });

  it('should not collapse a single executor', () => {
    const nodes = [view('view-1'), executor('exec-a'), view('view-2')];
    const edges = [successEdge('e1', 'view-1', 'exec-a'), successEdge('e2', 'exec-a', 'view-2')];

    const result = collapseExecutorChains(nodes, edges);

    expect(result.nodes).toEqual(nodes);
    expect(result.edges).toEqual(edges);
    expect(result.stackMembersById.size).toBe(0);
  });

  it('should not stack executors that have a failure branch', () => {
    const nodes = [view('view-1'), executor('exec-a'), executor('exec-b'), view('view-2'), view('view-err')];
    const edges = [
      successEdge('e1', 'view-1', 'exec-a'),
      successEdge('e2', 'exec-a', 'exec-b'),
      successEdge('e3', 'exec-b', 'view-2'),
      {id: 'e-fail', source: 'exec-a', sourceHandle: 'failure', target: 'view-err'},
    ];

    const result = collapseExecutorChains(nodes, edges);

    expect(result.nodes.some((node) => node.type === EXECUTION_STACK_NODE_TYPE)).toBe(false);
  });

  it('should not absorb a member that has a second incoming edge', () => {
    // exec-b is also reachable from view-2, so exec-a -> exec-b must not merge
    const nodes = [view('view-1'), executor('exec-a'), executor('exec-b'), view('view-2'), view('view-3')];
    const edges = [
      successEdge('e1', 'view-1', 'exec-a'),
      successEdge('e2', 'exec-a', 'exec-b'),
      successEdge('e3', 'exec-b', 'view-3'),
      successEdge('e4', 'view-2', 'exec-b'),
    ];

    const result = collapseExecutorChains(nodes, edges);

    expect(result.nodes.some((node) => node.type === EXECUTION_STACK_NODE_TYPE)).toBe(false);
  });

  it('should keep multiple incoming edges on the chain head', () => {
    const nodes = [view('view-1'), view('view-2'), executor('exec-a'), executor('exec-b'), view('view-3')];
    const edges = [
      successEdge('e1', 'view-1', 'exec-a'),
      successEdge('e2', 'view-2', 'exec-a'),
      successEdge('e3', 'exec-a', 'exec-b'),
      successEdge('e4', 'exec-b', 'view-3'),
    ];

    const result = collapseExecutorChains(nodes, edges);

    const stack = result.nodes.find((node) => node.type === EXECUTION_STACK_NODE_TYPE);
    expect(stack).toBeDefined();
    const retargeted = result.edges.filter((edge) => edge.target === stack?.id);
    expect(retargeted.map((edge) => edge.id).sort()).toEqual(['e1', 'e2']);
  });

  it('should leave a chain uncollapsed when its stack id is expanded', () => {
    const nodes = [view('view-1'), executor('exec-a'), executor('exec-b'), view('view-2')];
    const edges = [
      successEdge('e1', 'view-1', 'exec-a'),
      successEdge('e2', 'exec-a', 'exec-b'),
      successEdge('e3', 'exec-b', 'view-2'),
    ];

    const result = collapseExecutorChains(nodes, edges, new Set(['execution-stack_exec-a']));

    expect(result.nodes).toEqual(nodes);
    expect(result.edges).toEqual(edges);
    expect(result.stackMembersById.size).toBe(0);
  });

  it('should mark the stack selected only when every member is selected', () => {
    const nodes = [
      view('view-1'),
      {...executor('exec-a'), selected: true},
      {...executor('exec-b'), selected: true},
      view('view-2'),
    ];
    const edges = [
      successEdge('e1', 'view-1', 'exec-a'),
      successEdge('e2', 'exec-a', 'exec-b'),
      successEdge('e3', 'exec-b', 'view-2'),
    ];

    const result = collapseExecutorChains(nodes, edges);
    const stack = result.nodes.find((node) => node.type === EXECUTION_STACK_NODE_TYPE);
    expect(stack?.selected).toBe(true);
  });
});

describe('getExecutionStackWidth', () => {
  it('should grow by the layer offset per extra member', () => {
    expect(getExecutionStackWidth(1)).toBe(48);
    expect(getExecutionStackWidth(2)).toBe(52);
    expect(getExecutionStackWidth(3)).toBe(56);
  });

  it('should cap at the max deck layers', () => {
    expect(getExecutionStackWidth(4)).toBe(56);
    expect(getExecutionStackWidth(10)).toBe(56);
  });
});
