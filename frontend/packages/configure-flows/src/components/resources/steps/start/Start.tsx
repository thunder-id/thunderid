// Copyright 2023-2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Fab} from '@wso2/oxygen-ui';
import {Handle, Position} from '@xyflow/react';
import type {ReactElement} from 'react';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import {StaticStepTypes} from '../../../../models/steps';
import type {CommonStepFactoryPropsInterface} from '../CommonStepFactory';

/**
 * Props interface of {@link Start}
 */
export type StartPropsInterface = CommonStepFactoryPropsInterface;

/**
 * Start Node component.
 * This is a custom node supported by react flow renderer library.
 * See {@link https://reactflow.dev/docs/api/node-types/} for its documentation
 * and {@link https://reactflow.dev/examples/custom-node/} for an example
 *
 * @param props - Props injected to the component.
 * @returns Start node component.
 */
function Start(): ReactElement {
  return (
    <div>
      <Fab
        aria-label="start"
        color="primary"
        variant="extended"
        size="small"
        data-flow-node-surface
        sx={{boxShadow: 'none', pointerEvents: 'none', zIndex: 0}}
      >
        Start
      </Fab>
      <Handle
        id={`${StaticStepTypes.Start.toLowerCase()}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        type="source"
        position={Position.Right}
      />
    </div>
  );
}

export default Start;
