// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call */

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import VisualFlow from '../VisualFlow';

// Mock @xyflow/react
vi.mock('@xyflow/react', () => ({
  ReactFlow: ({children, nodes, edges, colorMode, deleteKeyCode, snapToGrid, snapGrid}: any) => (
    <div
      data-testid="react-flow"
      data-nodes={JSON.stringify(nodes)}
      data-edges={JSON.stringify(edges)}
      data-color-mode={colorMode}
      data-delete-key-code={JSON.stringify(deleteKeyCode)}
      data-snap-to-grid={String(snapToGrid)}
      data-snap-grid={JSON.stringify(snapGrid)}
    >
      {children}
    </div>
  ),
  Background: ({gap}: any) => <div data-testid="react-flow-background" data-gap={gap} />,
  MiniMap: ({pannable, zoomable, position, nodeColor}: any) => (
    <div
      data-testid="react-flow-minimap"
      data-pannable={String(pannable)}
      data-zoomable={String(zoomable)}
      data-position={position}
      data-color-start={nodeColor?.({type: 'START'})}
      data-color-view={nodeColor?.({type: 'VIEW'})}
      data-color-end={nodeColor?.({type: 'END'})}
      data-color-other={nodeColor?.({type: 'TASK_EXECUTION'})}
    />
  ),
}));

// Mock color scheme - allow modification for tests
let mockColorSchemeMode = 'light';
let mockColorSchemeSystemMode = 'light';

// Mock @wso2/oxygen-ui
vi.mock('@wso2/oxygen-ui', () => ({
  useColorScheme: () => ({
    mode: mockColorSchemeMode,
    systemMode: mockColorSchemeSystemMode,
  }),
}));

describe('VisualFlow', () => {
  const mockOnNodesChange = vi.fn();
  const mockOnEdgesChange = vi.fn();
  const mockOnConnect = vi.fn();
  const mockOnNodesDelete = vi.fn();
  const mockOnEdgesDelete = vi.fn();
  const mockOnNodeDragStop = vi.fn();

  const defaultProps = {
    nodes: [],
    edges: [],
    onNodesChange: mockOnNodesChange,
    onEdgesChange: mockOnEdgesChange,
    onConnect: mockOnConnect,
    onNodesDelete: mockOnNodesDelete,
    onEdgesDelete: mockOnEdgesDelete,
    onNodeDragStop: mockOnNodeDragStop,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockColorSchemeMode = 'light';
    mockColorSchemeSystemMode = 'light';
  });

  describe('Rendering', () => {
    it('should render ReactFlow component', () => {
      render(<VisualFlow {...defaultProps} />);

      expect(screen.getByTestId('react-flow')).toBeInTheDocument();
    });

    it('should render Background component', () => {
      render(<VisualFlow {...defaultProps} />);

      const background = screen.getByTestId('react-flow-background');
      expect(background).toBeInTheDocument();
      expect(background).toHaveAttribute('data-gap', '20');
    });
  });

  describe('Nodes and Edges', () => {
    it('should pass nodes to ReactFlow', () => {
      const nodes = [
        {id: 'node-1', position: {x: 0, y: 0}, data: {label: 'Node 1'}},
        {id: 'node-2', position: {x: 100, y: 100}, data: {label: 'Node 2'}},
      ];

      render(<VisualFlow {...defaultProps} nodes={nodes} />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-nodes', JSON.stringify(nodes));
    });

    it('should pass edges to ReactFlow', () => {
      const edges = [{id: 'edge-1', source: 'node-1', target: 'node-2'}];

      render(<VisualFlow {...defaultProps} edges={edges} />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-edges', JSON.stringify(edges));
    });

    it('should handle empty nodes and edges', () => {
      render(<VisualFlow {...defaultProps} nodes={[]} edges={[]} />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-nodes', '[]');
      expect(reactFlow).toHaveAttribute('data-edges', '[]');
    });
  });

  describe('Color Mode', () => {
    it('should pass color mode to ReactFlow', () => {
      render(<VisualFlow {...defaultProps} />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-color-mode', 'light');
    });

    it('should use systemMode when mode is system', () => {
      mockColorSchemeMode = 'system';
      mockColorSchemeSystemMode = 'dark';

      render(<VisualFlow {...defaultProps} />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-color-mode', 'dark');
    });

    it('should use mode directly when mode is dark', () => {
      mockColorSchemeMode = 'dark';
      mockColorSchemeSystemMode = 'light';

      render(<VisualFlow {...defaultProps} />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-color-mode', 'dark');
    });
  });

  describe('Canvas Editing Accelerators', () => {
    it('should register both Backspace and Delete as delete keys', () => {
      render(<VisualFlow {...defaultProps} />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-delete-key-code', JSON.stringify(['Backspace', 'Delete']));
    });

    it('should not snap to grid by default', () => {
      render(<VisualFlow {...defaultProps} />);

      expect(screen.getByTestId('react-flow')).toHaveAttribute('data-snap-to-grid', 'false');
    });

    it('should snap to the background grid when enabled', () => {
      render(<VisualFlow {...defaultProps} snapToGrid />);

      const reactFlow = screen.getByTestId('react-flow');
      expect(reactFlow).toHaveAttribute('data-snap-to-grid', 'true');
      expect(reactFlow).toHaveAttribute('data-snap-grid', JSON.stringify([20, 20]));
    });
  });

  describe('MiniMap', () => {
    it('should not render the minimap by default', () => {
      render(<VisualFlow {...defaultProps} />);

      expect(screen.queryByTestId('react-flow-minimap')).not.toBeInTheDocument();
    });

    it('should render a pannable, zoomable minimap when enabled', () => {
      render(<VisualFlow {...defaultProps} showMiniMap />);

      const miniMap = screen.getByTestId('react-flow-minimap');
      expect(miniMap).toBeInTheDocument();
      expect(miniMap).toHaveAttribute('data-pannable', 'true');
      expect(miniMap).toHaveAttribute('data-zoomable', 'true');
      expect(miniMap).toHaveAttribute('data-position', 'bottom-right');
    });

    it('should color minimap nodes by step type', () => {
      render(<VisualFlow {...defaultProps} showMiniMap />);

      const miniMap = screen.getByTestId('react-flow-minimap');
      expect(miniMap).toHaveAttribute('data-color-start', 'var(--oxygen-palette-success-main)');
      expect(miniMap).toHaveAttribute('data-color-view', 'var(--oxygen-palette-primary-main)');
      expect(miniMap).toHaveAttribute('data-color-end', 'var(--oxygen-palette-error-main)');
      expect(miniMap).toHaveAttribute('data-color-other', 'var(--oxygen-palette-action-disabled)');
    });
  });

  describe('Custom Node and Edge Types', () => {
    it('should accept custom nodeTypes', () => {
      const customNodeTypes = {
        customNode: () => <div>Custom Node</div>,
      };

      render(<VisualFlow {...defaultProps} nodeTypes={customNodeTypes} />);

      expect(screen.getByTestId('react-flow')).toBeInTheDocument();
    });

    it('should accept custom edgeTypes', () => {
      const customEdgeTypes = {
        customEdge: () => <div>Custom Edge</div>,
      };

      render(<VisualFlow {...defaultProps} edgeTypes={customEdgeTypes} />);

      expect(screen.getByTestId('react-flow')).toBeInTheDocument();
    });

    it('should default to empty objects for node and edge types', () => {
      render(<VisualFlow {...defaultProps} />);

      expect(screen.getByTestId('react-flow')).toBeInTheDocument();
    });
  });
});
