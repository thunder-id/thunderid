// Copyright 2023-2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useColorScheme} from '@wso2/oxygen-ui';
import {Background, type EdgeTypes, type NodeTypes, ReactFlow, type ReactFlowProps} from '@xyflow/react';
import type {CSSProperties, ReactElement} from 'react';
import '@xyflow/react/dist/style.css';

/**
 * Props interface of {@link VisualFlow}
 */
export interface VisualFlowPropsInterface extends ReactFlowProps {
  /**
   * Edge types to be rendered.
   */
  edgeTypes?: EdgeTypes;
  /**
   * Node types to be rendered.
   */
  nodeTypes?: NodeTypes;
}

/**
 * Wrapper component for React Flow used in the Visual Editor.
 *
 * @param props - Props injected to the component.
 * @returns Visual editor flow component.
 */
function VisualFlow({
  nodeTypes = {},
  edgeTypes = {},
  nodes,
  onNodesChange,
  edges,
  onEdgesChange,
  onConnect,
  onNodesDelete,
  onEdgesDelete,
  onNodeDragStop,
  onNodeClick,
  onEdgeClick,
  onEdgeMouseEnter,
  onEdgeMouseLeave,
}: VisualFlowPropsInterface): ReactElement {
  const {mode, systemMode} = useColorScheme();

  // Determine the effective color mode for React Flow
  const colorMode = mode === 'system' ? systemMode : mode;

  return (
    <ReactFlow
      fitView
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onConnect={onConnect}
      onNodesDelete={onNodesDelete}
      onEdgesDelete={onEdgesDelete}
      onNodeDragStop={onNodeDragStop}
      onNodeClick={onNodeClick}
      onEdgeClick={onEdgeClick}
      onEdgeMouseEnter={onEdgeMouseEnter}
      onEdgeMouseLeave={onEdgeMouseLeave}
      proOptions={{hideAttribution: true}}
      colorMode={colorMode}
      minZoom={0.2}
      maxZoom={4}
      style={
        {
          '--xy-background-color-default': colorMode === 'dark' ? '#0000002b' : '#ffffff2b',
          '--xy-edge-stroke': '#d7d7d7',
        } as CSSProperties
      }
    >
      <Background gap={20} />
    </ReactFlow>
  );
}

export default VisualFlow;
