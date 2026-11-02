// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment */

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import VisualFlow from '../VisualFlow';

// Mock @xyflow/react
vi.mock('@xyflow/react', () => ({
  ReactFlow: ({children, nodes, edges, colorMode}: any) => (
    <div
      data-testid="react-flow"
      data-nodes={JSON.stringify(nodes)}
      data-edges={JSON.stringify(edges)}
      data-color-mode={colorMode}
    >
      {children}
    </div>
  ),
  Background: ({gap}: any) => <div data-testid="react-flow-background" data-gap={gap} />,
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
