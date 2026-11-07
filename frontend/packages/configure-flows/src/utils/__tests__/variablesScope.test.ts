// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import findVariablesNodesBefore from '../variablesScope';

const node = (id: string, type = 'TASK_EXECUTION'): Node => ({id, type, position: {x: 0, y: 0}, data: {}});
const edge = (source: string, target: string): Edge => ({id: `${source}-${target}`, source, target});

describe('findVariablesNodesBefore', () => {
  it('finds the variables node every route passes before the step', () => {
    const nodes = [node('start', 'START'), node('vars', 'VARIABLES'), node('step'), node('end', 'END')];
    const edges = [edge('start', 'vars'), edge('vars', 'step'), edge('step', 'end')];

    expect(findVariablesNodesBefore('step', nodes, edges)).toEqual(['vars']);
  });

  it('lists the nearest variables node first', () => {
    const nodes = [node('start', 'START'), node('far', 'VARIABLES'), node('near', 'VARIABLES'), node('step')];
    const edges = [edge('start', 'far'), edge('far', 'near'), edge('near', 'step')];

    expect(findVariablesNodesBefore('step', nodes, edges)).toEqual(['near', 'far']);
  });

  it('leaves out a variables node a route can bypass on the way to the step', () => {
    const nodes = [node('start', 'START'), node('vars', 'VARIABLES'), node('other'), node('step')];
    const edges = [edge('start', 'vars'), edge('vars', 'step'), edge('start', 'other'), edge('other', 'step')];

    expect(findVariablesNodesBefore('step', nodes, edges)).toEqual([]);
  });

  it('leaves out a variables node that comes after the step, or sits on another branch', () => {
    const nodes = [node('start', 'START'), node('step'), node('after', 'VARIABLES'), node('side', 'VARIABLES')];
    const edges = [edge('start', 'step'), edge('step', 'after'), edge('start', 'side')];

    expect(findVariablesNodesBefore('step', nodes, edges)).toEqual([]);
  });

  it('counts a variables node that every branch passes after the routes rejoin', () => {
    const nodes = [node('start', 'START'), node('a'), node('b'), node('vars', 'VARIABLES'), node('step')];
    const edges = [edge('start', 'a'), edge('start', 'b'), edge('a', 'vars'), edge('b', 'vars'), edge('vars', 'step')];

    expect(findVariablesNodesBefore('step', nodes, edges)).toEqual(['vars']);
  });

  it('finds nothing for a step no route reaches, or when the flow has no start', () => {
    expect(
      findVariablesNodesBefore('step', [node('start', 'START'), node('step'), node('vars', 'VARIABLES')], []),
    ).toEqual([]);
    expect(findVariablesNodesBefore('step', [node('vars', 'VARIABLES'), node('step')], [edge('vars', 'step')])).toEqual(
      [],
    );
  });
});
