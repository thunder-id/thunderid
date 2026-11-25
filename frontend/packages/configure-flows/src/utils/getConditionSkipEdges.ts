// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {MarkerType, type Edge, type Node} from '@xyflow/react';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import {StepTypes, type StepData} from '../models/steps';

/**
 * Derives display-only edges from node conditions.
 */
export default function getConditionSkipEdges(nodes: Node[], label: string, edgeType: string): Edge[] {
  const nodeIds = new Set(nodes.map(({id}) => id));
  return nodes.flatMap(({id, data, type}): Edge[] => {
    const target = (data as StepData).condition?.onSkip;
    if (!target || !nodeIds.has(target)) {
      return [];
    }

    return [
      {
        id: `condition-skip:${id}`,
        source: id,
        sourceHandle:
          type === StepTypes.Execution ? `${id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}` : null,
        target,
        type: edgeType,
        label,
        ariaLabel: label,
        data: {conditionSkip: true},
        deletable: false,
        reconnectable: false,
        selectable: false,
        markerEnd: {type: MarkerType.Arrow},
        style: {strokeDasharray: '5 4'},
      },
    ];
  });
}
