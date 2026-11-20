// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import type {Step} from '../../models/steps';
import type {Widget} from '../../models/widget';
import {placeWidgetCluster, widgetNeedsViewContainer} from '../widgetUtils';

const createMockWidget = (steps?: {__generationMeta__?: {strategy?: string}}[]): Widget =>
  ({
    id: 'widget-1',
    category: 'COMPOSITE',
    type: 'IDENTIFIER_PASSWORD',
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    resourceType: 'WIDGET',
    display: {
      label: 'Widget',
      image: '',
      showOnResourcePanel: true,
    },
    config: {
      field: {name: '', type: {}},
      styles: {},
      data: {
        steps,
      },
    },
  }) as Widget;

describe('widgetNeedsViewContainer', () => {
  it('returns true when a widget has no generated steps', () => {
    expect(widgetNeedsViewContainer(createMockWidget([]))).toBe(true);
  });

  it('returns true when any step uses the merge-with-drop-point strategy', () => {
    expect(
      widgetNeedsViewContainer(
        createMockWidget([
          {__generationMeta__: {strategy: 'MERGE_WITH_DROP_POINT'}},
          {__generationMeta__: {strategy: 'APPEND'}},
        ]),
      ),
    ).toBe(true);
  });

  it('returns false when all steps omit the merge-with-drop-point strategy', () => {
    expect(
      widgetNeedsViewContainer(
        createMockWidget([{__generationMeta__: {strategy: 'APPEND'}}, {__generationMeta__: {strategy: 'INLINE'}}]),
      ),
    ).toBe(false);
  });

  it('returns false when all steps are missing generation metadata', () => {
    expect(widgetNeedsViewContainer(createMockWidget([{}, {}]))).toBe(false);
  });
});

describe('placeWidgetCluster', () => {
  const widgetSteps = [
    {id: '{{A}}', position: {x: 760, y: 360}, size: {width: 200, height: 113}},
    {id: '{{B}}', position: {x: 1120, y: 240}, size: {width: 360, height: 260}},
  ] as unknown as Step[];

  const node = (id: string, x: number, y: number, measured?: {width: number; height: number}): Node =>
    ({id, position: {x, y}, data: {}, measured}) as Node;

  it('keeps the widget positions when nothing overlaps them', () => {
    const preExisting: Node[] = [node('start', 0, 0, {width: 100, height: 50})];
    const resultNodes: Node[] = [...preExisting, node('a', 760, 360), node('b', 1120, 240)];
    const resolved = new Map([
      ['A', 'a'],
      ['B', 'b'],
    ]);

    expect(placeWidgetCluster(preExisting, resultNodes, widgetSteps, resolved)).toBe(resultNodes);
  });

  it('moves the widget nodes below the existing nodes they would overlap', () => {
    // A first copy of the same widget already sits at the widget's fixed positions.
    const preExisting: Node[] = [
      node('a1', 760, 360, {width: 200, height: 113}),
      node('b1', 1120, 240, {width: 360, height: 260}),
    ];
    const resultNodes: Node[] = [...preExisting, node('a2', 760, 360), node('b2', 1120, 240)];

    const resolved = new Map([
      ['A', 'a2'],
      ['B', 'b2'],
    ]);

    const placed: Node[] = placeWidgetCluster(preExisting, resultNodes, widgetSteps, resolved);

    // Below the lowest overlapping node (240 + 260) plus the gap, keeping the cluster's own layout.
    expect(placed.find((n) => n.id === 'a2')?.position).toEqual({x: 760, y: 700});
    expect(placed.find((n) => n.id === 'b2')?.position).toEqual({x: 1120, y: 580});
    expect(placed.find((n) => n.id === 'a1')).toBe(preExisting[0]);
    expect(placed.find((n) => n.id === 'b1')).toBe(preExisting[1]);
  });

  it('ignores steps that merge into the drop point', () => {
    const mergeOnly = [
      {id: 'm', position: {x: 0, y: 0}, __generationMeta__: {strategy: 'MERGE_WITH_DROP_POINT'}},
    ] as unknown as Step[];
    const preExisting: Node[] = [node('view', 0, 0, {width: 300, height: 300})];
    const resultNodes: Node[] = [...preExisting];

    expect(placeWidgetCluster(preExisting, resultNodes, mergeOnly, new Map())).toBe(resultNodes);
  });

  it('ignores steps that auto-wiring dropped in favour of an existing node', () => {
    // Only the dropped step B would overlap the existing node, so the kept node A stays put.
    const preExisting: Node[] = [node('existing_b', 1120, 240, {width: 360, height: 260})];
    const resultNodes: Node[] = [...preExisting, node('a', 760, 360)];
    const resolved = new Map([
      ['A', 'a'],
      ['B', 'b'],
    ]);

    expect(placeWidgetCluster(preExisting, resultNodes, widgetSteps, resolved)).toBe(resultNodes);
  });
});
