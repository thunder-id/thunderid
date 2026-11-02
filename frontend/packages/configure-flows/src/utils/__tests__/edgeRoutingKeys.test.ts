// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactFlowState} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import {DRAGGING_OBSTACLES_KEY, selectObstaclesKey} from '../edgeRoutingKeys';

function makeState(nodes: unknown[]): ReactFlowState {
  return {nodes} as unknown as ReactFlowState;
}

describe('selectObstaclesKey', () => {
  it('should return the dragging sentinel while any node is dragging', () => {
    const state = makeState([
      {id: 'a', position: {x: 0, y: 0}, dragging: false},
      {id: 'b', position: {x: 10, y: 10}, dragging: true},
    ]);

    expect(selectObstaclesKey(state)).toBe(DRAGGING_OBSTACLES_KEY);
  });

  it('should encode rounded node bounds into the key', () => {
    const state = makeState([{id: 'a', position: {x: 10.4, y: 20.6}, measured: {width: 150.2, height: 50.5}}]);

    expect(selectObstaclesKey(state)).toBe('a:10,21,150x51');
  });

  it('should join multiple nodes and default missing sizes to zero', () => {
    const state = makeState([
      {id: 'a', position: {x: 0, y: 0}, measured: {width: 100, height: 40}},
      {id: 'b', position: {x: 200, y: 0}},
    ]);

    expect(selectObstaclesKey(state)).toBe('a:0,0,100x40|b:200,0,0x0');
  });

  it('should return an identical key for the same nodes array (cached)', () => {
    const nodes = [{id: 'a', position: {x: 1, y: 2}, measured: {width: 10, height: 10}}];
    const state = makeState(nodes);

    const first = selectObstaclesKey(state);
    const second = selectObstaclesKey(makeState(nodes));

    expect(second).toBe(first);
  });
});
