// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import {Handle, Position} from '@xyflow/react';
import type {ReactElement} from 'react';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import {executionSurfaceMixin, nodeShadowMixin} from '../flowNodeStyles';

/**
 * Props interface of {@link Variables}
 */
export interface VariablesPropsInterface {
  /**
   * The id of the node, which its source handle is named after.
   */
  id: string;
}

/**
 * Placeholder for a VARIABLES node, which the editor does not edit yet. It is a blank box that keeps the paths on
 * either side of it connected, so the rest of the flow is drawn. What the node declares is carried on the node and
 * written back unchanged on save.
 *
 * @param props - Props injected to the component.
 * @returns Variables placeholder component.
 */
function Variables({id}: VariablesPropsInterface): ReactElement {
  return (
    <Box
      data-flow-node-surface
      data-testid="variables-node"
      sx={[
        executionSurfaceMixin,
        nodeShadowMixin,
        {
          border: '1px dashed',
          borderColor: 'divider',
          borderRadius: 1,
          minHeight: 48,
          minWidth: 120,
        },
      ]}
    >
      <Handle type="target" position={Position.Left} />
      <Handle
        id={`${id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        type="source"
        position={Position.Right}
      />
    </Box>
  );
}

export default Variables;
