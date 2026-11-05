// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import VisualFlowConstants from '../../constants/VisualFlowConstants';
import {StepTypes} from '../../models/steps';
import getConditionSkipEdges from '../getConditionSkipEdges';

const condition = {key: '{{ctx(recipientType)}}', value: 'email', onSkip: 'skip_target'};

const node = (id: string, type: string, data: Record<string, unknown> = {}): Node => ({
  id,
  type,
  position: {x: 0, y: 0},
  data,
});

describe('getConditionSkipEdges', () => {
  it('should draw a read-only skip edge from the success handle of an execution node', () => {
    const nodes = [node('send_email', StepTypes.Execution, {condition}), node('skip_target', StepTypes.Execution)];

    const edges = getConditionSkipEdges(nodes, 'On skip', 'smoothstep');

    expect(edges).toHaveLength(1);
    expect(edges[0]).toMatchObject({
      id: 'condition-skip:send_email',
      source: 'send_email',
      sourceHandle: `send_email${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
      target: 'skip_target',
      type: 'smoothstep',
      label: 'On skip',
      data: {conditionSkip: true},
      deletable: false,
      reconnectable: false,
      selectable: false,
    });
  });

  it('should draw the skip edge from the node itself when it is not an execution node', () => {
    const nodes = [node('prompt', StepTypes.View, {condition}), node('skip_target', StepTypes.Execution)];

    expect(getConditionSkipEdges(nodes, 'On skip', 'smoothstep')[0]).toMatchObject({
      source: 'prompt',
      sourceHandle: null,
      target: 'skip_target',
    });
  });

  it('should omit a skip edge when its target is missing', () => {
    expect(
      getConditionSkipEdges([node('send_email', StepTypes.Execution, {condition})], 'On skip', 'smoothstep'),
    ).toEqual([]);
  });

  it('should draw nothing for a node without a condition', () => {
    const nodes = [node('send_email', StepTypes.Execution), node('skip_target', StepTypes.Execution)];

    expect(getConditionSkipEdges(nodes, 'On skip', 'smoothstep')).toEqual([]);
  });
});
