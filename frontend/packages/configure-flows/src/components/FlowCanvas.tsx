// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ReactFlowProvider} from '@xyflow/react';
import {type ReactElement} from 'react';
import DecoratedVisualFlow, {type DecoratedVisualFlowPropsInterface} from './visual-flow/DecoratedVisualFlow';

/**
 * Props interface of {@link FlowCanvas}
 */
export type FlowCanvasPropsInterface = DecoratedVisualFlowPropsInterface;

/**
 * Renders the flow builder canvas.
 *
 * @param props - Props injected to the component.
 * @returns FlowCanvas component.
 */
function FlowCanvas({...rest}: FlowCanvasPropsInterface): ReactElement {
  return (
    <ReactFlowProvider>
      <DecoratedVisualFlow {...rest} />
    </ReactFlowProvider>
  );
}

export default FlowCanvas;
