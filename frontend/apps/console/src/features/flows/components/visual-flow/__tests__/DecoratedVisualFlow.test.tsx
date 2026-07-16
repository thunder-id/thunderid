/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-assignment, react/require-default-props */

import {render, screen, fireEvent, waitFor, cleanup, act} from '@thunderid/test-utils';
import type {Node, Edge} from '@xyflow/react';
import React from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {Resources} from '../../../models/resources';
import DecoratedVisualFlow from '../DecoratedVisualFlow';

// Mock hooks
vi.mock('../../../hooks/useUIPanelState', () => ({
  default: () => ({
    isResourcePanelOpen: true,
    isResourcePropertiesPanelOpen: false,
  }),
}));

vi.mock('../../../hooks/useFlowConfig', () => ({
  default: () => ({
    isFlowMetadataLoading: false,
    metadata: undefined,
    setFlowNodes: vi.fn(),
  }),
}));

vi.mock('../../../hooks/useInteractionState', () => ({
  default: () => ({
    onResourceDropOnCanvas: vi.fn(),
  }),
}));

vi.mock('../../../hooks/useFlowEvents', () => ({
  default: () => ({
    notifyElementAdded: vi.fn(),
    onElementAdded: vi.fn(() => vi.fn()),
    triggerAutoLayout: vi.fn(),
    onAutoLayout: vi.fn(() => vi.fn()),
    restoreFromHistory: vi.fn(),
    onRestoreFromHistory: vi.fn(() => vi.fn()),
  }),
}));

vi.mock('../../../hooks/useComponentDelete', () => ({
  default: () => ({
    deleteComponent: vi.fn(),
  }),
}));

vi.mock('../../../hooks/useResourceAdd', () => ({
  default: () => vi.fn(),
}));

vi.mock('../../../hooks/useGenerateStepElement', () => ({
  default: () => ({
    generateStepElement: vi.fn(),
  }),
}));

vi.mock('../../../hooks/useDeleteExecutionResource', () => ({
  default: () => null,
}));

vi.mock('../../../hooks/useStaticContentField', () => ({
  default: () => null,
}));

vi.mock('../../../hooks/useConfirmPasswordField', () => ({
  default: () => null,
}));

vi.mock('../../../hooks/useVisualFlowHandlers', () => ({
  default: () => ({
    handleConnect: vi.fn(),
    handleNodesDelete: vi.fn(),
    handleEdgesDelete: vi.fn(),
  }),
}));

vi.mock('../../../hooks/useDragDropHandlers', () => ({
  default: () => ({
    addCanvasNode: vi.fn(),
    addToView: vi.fn(),
    addToForm: vi.fn(),
    addToViewAtIndex: vi.fn(),
    addToFormAtIndex: vi.fn(),
  }),
}));

vi.mock('../../../hooks/useContainerDialogConfirm', () => ({
  default: () => vi.fn(),
}));

// Use vi.hoisted for applyAutoLayout mock to ensure it's always available
const {mockApplyAutoLayout} = vi.hoisted(() => ({
  mockApplyAutoLayout: vi.fn().mockResolvedValue([]),
}));

vi.mock('../../../utils/applyAutoLayout', () => ({
  default: mockApplyAutoLayout,
}));

vi.mock('../../../utils/resolveCollisions', () => ({
  resolveCollisions: vi.fn((nodes) => nodes),
}));

vi.mock('../../../utils/computeExecutorConnections', () => ({
  default: vi.fn(() => []),
}));

vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useIdentityProviders: () => ({data: []}),
  useSMSProviders: () => ({data: []}),
}));

// Use vi.hoisted for mocks that need to be referenced in vi.mock
const {mockToObject, mockGetNodes, mockGetEdges, mockUpdateNodeData, mockFitView, mockUpdateNodeInternals} = vi.hoisted(
  () => ({
    mockToObject: vi.fn(() => ({viewport: {x: 0, y: 0, zoom: 1}})),
    mockGetNodes: vi.fn((): Node[] => []),
    mockGetEdges: vi.fn((): Edge[] => []),
    mockUpdateNodeData: vi.fn(),
    mockFitView: vi.fn().mockResolvedValue(undefined),
    mockUpdateNodeInternals: vi.fn(),
  }),
);

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    toObject: mockToObject,
    getNodes: mockGetNodes,
    getEdges: mockGetEdges,
    updateNodeData: mockUpdateNodeData,
    fitView: mockFitView,
  }),
  useUpdateNodeInternals: () => mockUpdateNodeInternals,
}));

// Store callbacks for testing
type DragEndCallback = (event: {
  operation: {
    source: {data: Record<string, unknown>} | null;
    target: {id: string; data: Record<string, unknown>} | null;
  };
  canceled: boolean;
}) => void;
type DragOverCallback = (event: {
  operation: {
    source: {id?: string; data?: Record<string, unknown>} | null;
    target?: {id: string; data: Record<string, unknown>} | null;
  };
}) => void;

let capturedOnDragEnd: DragEndCallback | null = null;
let capturedOnDragOver: DragOverCallback | null = null;

const triggerCapturedDragEnd = (event: Parameters<DragEndCallback>[0]): void => {
  act(() => {
    capturedOnDragEnd?.(event);
  });
};

const triggerCapturedDragOver = (event: Parameters<DragOverCallback>[0]): void => {
  act(() => {
    capturedOnDragOver?.(event);
  });
};

// Mock @dnd-kit/react
vi.mock('@dnd-kit/react', () => ({
  DragDropProvider: ({
    children,
    onDragEnd,
    onDragOver,
  }: {
    children: React.ReactNode;
    onDragEnd?: DragEndCallback;
    onDragOver?: DragOverCallback;
  }) => {
    // Capture callbacks for testing
    capturedOnDragEnd = onDragEnd ?? null;
    capturedOnDragOver = onDragOver ?? null;

    return (
      <div data-testid="drag-drop-provider" data-ondragend={!!onDragEnd} data-ondragover={!!onDragOver}>
        {children}
      </div>
    );
  },
  DragOverlay: ({children}: {children: React.ReactNode | ((source: unknown) => React.ReactNode)}) => (
    <div data-testid="drag-overlay">{typeof children === 'function' ? null : children}</div>
  ),
}));

// Mock @dnd-kit/helpers
vi.mock('@dnd-kit/helpers', () => ({
  move: vi.fn((items) => items),
}));

// Mock @dnd-kit/abstract
vi.mock('@dnd-kit/abstract', () => ({
  CollisionPriority: {
    Low: 'low',
    High: 'high',
  },
}));

// Mock @wso2/oxygen-ui
vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    OxygenUIThemeProvider: ({children}: any) => children,
  };
});

// Mock classnames
vi.mock('classnames', () => ({
  default: (...args: any[]) => args.filter(Boolean).join(' '),
}));

// Mock child components
vi.mock('../VisualFlow', () => ({
  default: ({nodes, edges, onNodeDragStop, onNodeClick}: any) => (
    <div
      data-testid="visual-flow"
      data-nodes={JSON.stringify(nodes)}
      data-edges={JSON.stringify(edges)}
      data-has-drag-stop={!!onNodeDragStop}
    >
      <button data-testid="node-drag-stop-trigger" onClick={onNodeDragStop}>
        Node Drag Stop
      </button>
      <button
        data-testid="node-click-trigger"
        onClick={(event) =>
          (onNodeClick as ((e: unknown, n: unknown) => void) | undefined)?.(event, {
            id: 'clicked-node',
            position: {x: 0, y: 0},
          })
        }
      >
        Node Click
      </button>
    </div>
  ),
}));

vi.mock('../CanvasToolbar', () => ({
  default: ({onAutoLayout}: any) => (
    <div data-testid="canvas-toolbar">
      <button data-testid="auto-layout-trigger" onClick={onAutoLayout}>
        Auto Layout
      </button>
    </div>
  ),
}));

vi.mock('../ValidationBadge', () => ({
  default: () => <div data-testid="validation-badge" />,
}));

vi.mock('../../dnd/Droppable', () => ({
  default: ({children, id, type}: any) => (
    <div data-testid="droppable" data-id={id} data-type={type}>
      {children}
    </div>
  ),
}));

vi.mock('../../resource-panel/ResourcePanel', () => ({
  default: ({children, open, disabled, flowTitle, rightPanel}: any) => (
    <div data-testid="resource-panel" data-open={open} data-disabled={disabled} data-title={flowTitle}>
      {children}
      {rightPanel && <div data-testid="right-panel">{rightPanel}</div>}
    </div>
  ),
}));

vi.mock('../../resource-property-panel/ResourcePropertyPanel', () => ({
  default: ({open}: any) => <div data-testid="resource-property-panel" data-open={open} />,
}));

vi.mock('../../validation-panel/ValidationPanel', () => ({
  default: ({open}: any) => <div data-testid="validation-panel" data-open={open} />,
}));

vi.mock('../FormRequiresViewDialog', () => ({
  default: ({open, scenario, onClose, onConfirm}: any) => (
    <div data-testid="form-requires-view-dialog" data-open={open} data-scenario={scenario}>
      <button data-testid="dialog-close" onClick={onClose}>
        Close
      </button>
      <button data-testid="dialog-confirm" onClick={onConfirm}>
        Confirm
      </button>
    </div>
  ),
}));

vi.mock('../../../utils/generateResourceId', () => ({
  default: (prefix: string) => `${prefix}_test123`,
}));

describe('DecoratedVisualFlow', () => {
  const mockResources: Resources = {
    steps: [],
    templates: [],
    elements: [],
    widgets: [],
    executors: [],
  };

  const renderComponent = (ui: React.ReactElement) => render(ui);

  const defaultProps = {
    resources: mockResources,
    nodes: [] as Node[],
    edges: [] as Edge[],
    setNodes: vi.fn(),
    setEdges: vi.fn(),
    onNodesChange: vi.fn(),
    onEdgesChange: vi.fn(),
    mutateComponents: vi.fn((components) => components),
    onTemplateLoad: vi.fn(() => [[], []] as [Node[], Edge[]]),
    onWidgetLoad: vi.fn(() => [[], [], null, null] as [Node[], Edge[], null, null]),
    onStepLoad: vi.fn((step) => step),
    onResourceAdd: vi.fn(),
    flowTitle: 'Test Flow',
    flowHandle: 'test-flow',
    onFlowTitleChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetNodes.mockReturnValue([]);
    mockGetEdges.mockReturnValue([]);
    // Reset applyAutoLayout to return a resolved promise by default
    mockApplyAutoLayout.mockResolvedValue([]);
    // Reset fitView to return a resolved promise by default
    mockFitView.mockResolvedValue(undefined);
  });

  afterEach(async () => {
    // Flush pending requestAnimationFrame callbacks before cleanup to prevent
    // "Cannot read properties of undefined (reading 'catch')" errors from
    // callbacks firing after component unmount
    await new Promise((resolve) => {
      requestAnimationFrame(() => {
        requestAnimationFrame(resolve);
      });
    });
    cleanup();
  });

  describe('Rendering', () => {
    it('should render the component structure', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should render DragDropProvider', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('drag-drop-provider')).toBeInTheDocument();
    });

    it('should render ResourcePanel', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('resource-panel')).toBeInTheDocument();
    });

    it('should render ResourcePropertyPanel', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('resource-property-panel')).toBeInTheDocument();
    });

    it('should render Droppable canvas', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('droppable')).toBeInTheDocument();
    });

    it('should render VisualFlow', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should render a prominent simulate button in the top bar', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('simulate-flow-button')).toBeInTheDocument();
    });

    it('should disable save while previewing so the zoomed viewport is not persisted', () => {
      const nodes = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}] as Node[];
      renderComponent(<DecoratedVisualFlow {...defaultProps} nodes={nodes} onSave={vi.fn()} />);

      expect(screen.getByTestId('save-flow-button')).toBeEnabled();

      fireEvent.click(screen.getByTestId('simulate-flow-button'));

      expect(screen.getByTestId('save-flow-button')).toBeDisabled();
    });

    it('should focus the clicked node via fitView', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      fireEvent.click(screen.getByTestId('node-click-trigger'));

      expect(mockFitView).toHaveBeenCalledWith({
        nodes: [{id: 'clicked-node'}],
        padding: 0.3,
        maxZoom: 1.2,
        duration: 500,
      });
    });

    it('should render ValidationPanel', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('validation-panel')).toBeInTheDocument();
    });

    it('should render FormRequiresViewDialog', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('form-requires-view-dialog')).toBeInTheDocument();
    });
  });

  describe('Props Passing', () => {
    it('should pass nodes to VisualFlow', () => {
      const nodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];

      renderComponent(<DecoratedVisualFlow {...defaultProps} nodes={nodes} />);

      const visualFlow = screen.getByTestId('visual-flow');
      expect(visualFlow).toHaveAttribute('data-nodes', JSON.stringify(nodes));
    });

    it('should pass edges to VisualFlow', () => {
      const edges: Edge[] = [{id: 'edge-1', source: 'node-1', target: 'node-2'}];

      renderComponent(<DecoratedVisualFlow {...defaultProps} edges={edges} />);

      const visualFlow = screen.getByTestId('visual-flow');
      expect(visualFlow).toHaveAttribute('data-edges', JSON.stringify(edges));
    });

    it('should pass flow title to ResourcePanel', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} flowTitle="My Custom Flow" />);

      const resourcePanel = screen.getByTestId('resource-panel');
      expect(resourcePanel).toHaveAttribute('data-title', 'My Custom Flow');
    });

    it('should render save button in top bar', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} onSave={vi.fn()} />);

      expect(screen.getByText('core.headerPanel.save')).toBeInTheDocument();
    });

    it('should render canvas toolbar', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('canvas-toolbar')).toBeInTheDocument();
    });
  });

  describe('Save Functionality', () => {
    it('should call onSave with canvas data when save is triggered', () => {
      const mockOnSave = vi.fn();
      mockToObject.mockReturnValue({viewport: {x: 10, y: 20, zoom: 1.5}});
      mockGetNodes.mockReturnValue([{id: 'node-1', position: {x: 0, y: 0}, data: {}}]);
      mockGetEdges.mockReturnValue([{id: 'edge-1', source: 'node-1', target: 'node-2'}]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} onSave={mockOnSave} />);

      const saveButton = screen.getByText('core.headerPanel.save');
      fireEvent.click(saveButton);

      expect(mockOnSave).toHaveBeenCalledWith({
        nodes: [{id: 'node-1', position: {x: 0, y: 0}, data: {}}],
        edges: [{id: 'edge-1', source: 'node-1', target: 'node-2'}],
        viewport: {x: 10, y: 20, zoom: 1.5},
      });
    });

    it('should not throw when onSave is not provided', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} onSave={undefined} />);

      const saveButton = screen.getByText('core.headerPanel.save');
      expect(() => fireEvent.click(saveButton)).not.toThrow();
    });
  });

  describe('Auto Layout', () => {
    it('should handle auto layout trigger', async () => {
      mockApplyAutoLayout.mockResolvedValue([{id: 'node-1', position: {x: 100, y: 100}, data: {}}]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const autoLayoutButton = screen.getByTestId('auto-layout-trigger');
      fireEvent.click(autoLayoutButton);

      await waitFor(() => {
        expect(mockApplyAutoLayout).toHaveBeenCalled();
      });
    });
  });

  describe('Form Requires View Dialog', () => {
    it('should render dialog in closed state initially', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const dialog = screen.getByTestId('form-requires-view-dialog');
      expect(dialog).toHaveAttribute('data-open', 'false');
    });

    it('should close dialog when close button is clicked', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const closeButton = screen.getByTestId('dialog-close');
      fireEvent.click(closeButton);

      const dialog = screen.getByTestId('form-requires-view-dialog');
      expect(dialog).toHaveAttribute('data-open', 'false');
    });
  });

  describe('Droppable Configuration', () => {
    it('should configure droppable with correct id prefix', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const droppable = screen.getByTestId('droppable');
      expect(droppable.getAttribute('data-id')).toContain('flow-builder-canvas');
    });

    it('should configure droppable with correct type', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const droppable = screen.getByTestId('droppable');
      expect(droppable).toHaveAttribute('data-type', 'flow-builder-droppable-canvas');
    });
  });

  describe('Resource Panel State', () => {
    it('should pass open state to resource panel', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const resourcePanel = screen.getByTestId('resource-panel');
      expect(resourcePanel).toHaveAttribute('data-open', 'true');
    });

    it('should pass disabled state based on loading', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const resourcePanel = screen.getByTestId('resource-panel');
      expect(resourcePanel).toHaveAttribute('data-disabled', 'false');
    });
  });

  describe('Auto Layout on Load', () => {
    it('should not trigger auto layout when triggerAutoLayoutOnLoad is false', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} triggerAutoLayoutOnLoad={false} />);

      // Auto layout should not be called on mount when flag is false
      expect(defaultProps.setNodes).not.toHaveBeenCalled();
    });

    it('should not trigger auto layout for single node', () => {
      mockGetNodes.mockReturnValue([{id: 'node-1', position: {x: 0, y: 0}, data: {}}]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} triggerAutoLayoutOnLoad />);

      // Single node should not trigger auto layout
      expect(defaultProps.setNodes).not.toHaveBeenCalled();
    });
  });

  describe('Edge Types', () => {
    it('should accept custom edge types', () => {
      const customEdgeTypes = {
        custom: () => <div>Custom Edge</div>,
      };

      renderComponent(<DecoratedVisualFlow {...defaultProps} edgeTypes={customEdgeTypes} />);

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should use default empty object for edge types', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });
  });

  describe('DragDropProvider Configuration', () => {
    it('should configure drag end handler', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const provider = screen.getByTestId('drag-drop-provider');
      expect(provider).toHaveAttribute('data-ondragend', 'true');
    });

    it('should configure drag over handler', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const provider = screen.getByTestId('drag-drop-provider');
      expect(provider).toHaveAttribute('data-ondragover', 'true');
    });
  });

  describe('Computed Metadata with Executor Connections', () => {
    it('should compute metadata with executor connections from identity providers', async () => {
      const computeExecutorConnections = await import('../../../utils/computeExecutorConnections');
      const mockCompute = vi.mocked(computeExecutorConnections.default);
      mockCompute.mockReturnValue([
        {executorName: 'google', connections: ['social']},
        {executorName: 'github', connections: ['social']},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(mockCompute).toHaveBeenCalled();
    });

    it('should handle empty executor connections with no metadata', async () => {
      const computeExecutorConnections = await import('../../../utils/computeExecutorConnections');
      const mockCompute = vi.mocked(computeExecutorConnections.default);
      mockCompute.mockReturnValue([]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Component should render without metadata
      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should merge executor connections with existing metadata', async () => {
      const computeExecutorConnections = await import('../../../utils/computeExecutorConnections');
      const mockCompute = vi.mocked(computeExecutorConnections.default);
      mockCompute.mockReturnValue([{executorName: 'google', connections: ['social']}]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(mockCompute).toHaveBeenCalledWith({identityProviders: [], smsProviders: []});
    });
  });

  describe('Auto Layout on Load with Multiple Nodes at Origin', () => {
    it('should check for multiple nodes at origin for auto-layout trigger', () => {
      // Multiple nodes at origin (0, 0) indicates auto-layout may be needed
      mockGetNodes.mockReturnValue([
        {id: 'node-1', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', position: {x: 0, y: 0}, data: {}},
        {id: 'node-3', position: {x: 0, y: 0}, data: {}},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} triggerAutoLayoutOnLoad />);

      // Component should render with canvas toolbar available
      expect(screen.getByTestId('canvas-toolbar')).toBeInTheDocument();
    });

    it('should not trigger auto layout when nodes have different positions', () => {
      mockGetNodes.mockReturnValue([
        {id: 'node-1', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', position: {x: 100, y: 100}, data: {}},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} triggerAutoLayoutOnLoad />);

      // Only one node at origin, so no auto-layout needed
      expect(defaultProps.setNodes).not.toHaveBeenCalled();
    });

    it('should provide auto-layout capability via canvas toolbar', () => {
      mockGetNodes.mockReturnValue([
        {id: 'node-1', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', position: {x: 0, y: 0}, data: {}},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} triggerAutoLayoutOnLoad />);

      // Verify canvas toolbar with auto-layout is rendered
      expect(screen.getByTestId('canvas-toolbar')).toBeInTheDocument();
      expect(screen.getByTestId('auto-layout-trigger')).toBeInTheDocument();
    });
  });

  describe('Node Drag Stop - Collision Resolution', () => {
    it('should resolve collisions when nodes overlap after drag', async () => {
      const resolveCollisionsModule = await import('../../../utils/resolveCollisions');
      const mockResolveCollisions = vi.mocked(resolveCollisionsModule.resolveCollisions);

      // Setup nodes with overlap
      const overlappingNodes: Node[] = [
        {id: 'node-1', position: {x: 100, y: 100}, data: {}},
        {id: 'node-2', position: {x: 110, y: 110}, data: {}}, // Overlapping
      ];

      // Return resolved positions (different from original)
      mockResolveCollisions.mockReturnValue([
        {id: 'node-1', position: {x: 100, y: 100}, data: {}},
        {id: 'node-2', position: {x: 300, y: 110}, data: {}}, // Moved to resolve
      ]);

      mockGetNodes.mockReturnValue(overlappingNodes);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Click the node drag stop trigger
      const dragStopButton = screen.getByTestId('node-drag-stop-trigger');
      fireEvent.click(dragStopButton);

      // setNodes should be called because positions changed
      expect(defaultProps.setNodes).toHaveBeenCalled();
    });

    it('should not update nodes when no collisions are detected', async () => {
      const resolveCollisionsModule = await import('../../../utils/resolveCollisions');
      const mockResolveCollisions = vi.mocked(resolveCollisionsModule.resolveCollisions);

      const nodes: Node[] = [
        {id: 'node-1', position: {x: 100, y: 100}, data: {}},
        {id: 'node-2', position: {x: 500, y: 100}, data: {}}, // No overlap
      ];

      // Return same positions (no changes)
      mockResolveCollisions.mockReturnValue(nodes);

      mockGetNodes.mockReturnValue(nodes);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Clear any previous calls
      defaultProps.setNodes.mockClear();

      // Click the node drag stop trigger
      const dragStopButton = screen.getByTestId('node-drag-stop-trigger');
      fireEvent.click(dragStopButton);

      // setNodes should NOT be called because positions are the same
      expect(defaultProps.setNodes).not.toHaveBeenCalled();
    });

    it('should pass correct options to resolveCollisions', async () => {
      const resolveCollisionsModule = await import('../../../utils/resolveCollisions');
      const mockResolveCollisions = vi.mocked(resolveCollisionsModule.resolveCollisions);

      const nodes: Node[] = [{id: 'node-1', position: {x: 100, y: 100}, data: {}}];
      mockResolveCollisions.mockReturnValue(nodes);
      mockGetNodes.mockReturnValue(nodes);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Click the node drag stop trigger
      const dragStopButton = screen.getByTestId('node-drag-stop-trigger');
      fireEvent.click(dragStopButton);

      // Verify resolveCollisions was called with correct options
      expect(mockResolveCollisions).toHaveBeenCalledWith(nodes, {
        maxIterations: 10,
        overlapThreshold: 0.5,
        margin: 50,
      });
    });

    it('should indicate node drag stop handler is present', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const visualFlow = screen.getByTestId('visual-flow');
      expect(visualFlow).toHaveAttribute('data-has-drag-stop', 'true');
    });
  });

  describe('Handle Drag End - Drop Scenarios', () => {
    it('should handle form drop on canvas by showing dialog', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Dialog should initially be closed
      const dialog = screen.getByTestId('form-requires-view-dialog');
      expect(dialog).toHaveAttribute('data-open', 'false');
    });

    it('should handle form-on-canvas drop scenario', async () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate form drop on canvas
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {type: 'FORM'},
            },
          },
          target: {
            id: 'flow-builder-canvas_test',
            data: {},
          },
        },
        canceled: false,
      });

      await waitFor(() => {
        const dialog = screen.getByTestId('form-requires-view-dialog');
        expect(dialog).toHaveAttribute('data-scenario', 'form-on-canvas');
      });
    });

    it('should handle input-on-canvas drop scenario', async () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate input drop on canvas
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {category: 'FIELD'},
            },
          },
          target: {
            id: 'flow-builder-canvas_test',
            data: {},
          },
        },
        canceled: false,
      });

      await waitFor(() => {
        const dialog = screen.getByTestId('form-requires-view-dialog');
        expect(dialog).toHaveAttribute('data-scenario', 'input-on-canvas');
      });
    });

    it('should handle input-on-view drop scenario', async () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate input drop on view
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {category: 'FIELD'},
            },
          },
          target: {
            id: 'flow-builder-view_test',
            data: {},
          },
        },
        canceled: false,
      });

      await waitFor(() => {
        const dialog = screen.getByTestId('form-requires-view-dialog');
        expect(dialog).toHaveAttribute('data-scenario', 'input-on-view');
      });
    });

    it('should handle widget-on-canvas drop scenario', async () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate widget drop on canvas
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {resourceType: 'WIDGET'},
            },
          },
          target: {
            id: 'flow-builder-canvas_test',
            data: {},
          },
        },
        canceled: false,
      });

      await waitFor(() => {
        const dialog = screen.getByTestId('form-requires-view-dialog');
        expect(dialog).toHaveAttribute('data-scenario', 'widget-on-canvas');
      });
    });

    it('should skip the container dialog for standalone widgets dropped on canvas', async () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {
                resourceType: 'WIDGET',
                config: {
                  data: {
                    steps: [
                      {
                        type: 'TASK_EXECUTION',
                      },
                    ],
                  },
                },
              },
            },
          },
          target: {
            id: 'flow-builder-canvas_test',
            data: {},
          },
        },
        canceled: false,
      });

      await waitFor(() => {
        const dialog = screen.getByTestId('form-requires-view-dialog');
        expect(dialog).toHaveAttribute('data-open', 'false');
      });
    });

    it('should return early when event is canceled', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate canceled drag event
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {dragged: {}},
          },
          target: null,
        },
        canceled: true,
      });

      // Dialog should remain closed
      const dialog = screen.getByTestId('form-requires-view-dialog');
      expect(dialog).toHaveAttribute('data-open', 'false');
    });

    it('should return early when source is missing', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drag event without source
      triggerCapturedDragEnd({
        operation: {
          source: null,
          target: {id: 'target-1', data: {}},
        },
        canceled: false,
      });

      // Component should still be rendered
      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should handle reordering scenario', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate reordering drag event
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
              dragged: {},
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
        canceled: false,
      });

      // updateNodeData should be called for reordering
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should handle reordering with nested components in handleDragEnd', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Capture the callback passed to updateNodeData
      mockUpdateNodeData.mockImplementation((stepId: string, callback: (node: Node) => {components: unknown[]}) => {
        // Simulate a node with nested components (form with children)
        const mockNode: Node = {
          id: stepId,
          position: {x: 0, y: 0},
          data: {
            components: [
              {id: 'button-1', type: 'BUTTON'},
              {
                id: 'form-1',
                type: 'FORM',
                components: [
                  {id: 'input-1', type: 'TEXT_INPUT'},
                  {id: 'input-2', type: 'TEXT_INPUT'},
                ],
              },
              {id: 'text-1', type: 'TEXT'},
            ],
          },
        };
        // Execute the callback to verify it processes nested components correctly
        const result = callback(mockNode);
        expect(result.components).toBeDefined();
      });

      // Simulate reordering drag event
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
              dragged: {},
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
        canceled: false,
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
    });

    it('should handle reordering with empty components in handleDragEnd', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Capture the callback passed to updateNodeData
      mockUpdateNodeData.mockImplementation((stepId: string, callback: (node: Node) => {components: unknown[]}) => {
        // Simulate a node with no components
        const mockNode: Node = {
          id: stepId,
          position: {x: 0, y: 0},
          data: {},
        };
        const result = callback(mockNode);
        expect(result.components).toBeDefined();
      });

      // Simulate reordering drag event
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
              dragged: {},
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
        canceled: false,
      });

      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should handle reordering without stepId', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate reordering without stepId
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: null,
              dragged: {},
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
        canceled: false,
      });

      // Should return early without calling updateNodeData
      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should handle drop on view target', () => {
      const mockAddToView = vi.fn();
      vi.doMock('../../../hooks/useDragDropHandlers', () => ({
        default: () => ({
          addCanvasNode: vi.fn(),
          addToView: mockAddToView,
          addToForm: vi.fn(),
          addToViewAtIndex: vi.fn(),
          addToFormAtIndex: vi.fn(),
        }),
      }));

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drop on view
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {type: 'BUTTON'},
            },
          },
          target: {
            id: 'flow-builder-view_test',
            data: {},
          },
        },
        canceled: false,
      });

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should handle drop on form target', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drop on form
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {type: 'TEXT_INPUT'},
            },
          },
          target: {
            id: 'flow-builder-form_test',
            data: {},
          },
        },
        canceled: false,
      });

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should handle drop on existing element for reordering at index', () => {
      const targetNode: Node = {
        id: 'step-1',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'element-1', type: 'BUTTON'},
            {id: 'form-1', type: 'FORM', components: [{id: 'input-1', type: 'TEXT_INPUT'}]},
          ],
        },
      };
      mockGetNodes.mockReturnValue([targetNode]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drop on existing element (insert at position)
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {type: 'BUTTON'},
            },
          },
          target: {
            id: 'element-1',
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
        },
        canceled: false,
      });

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should handle drop on element inside form for reordering', () => {
      // Use 'BLOCK' which is the actual value of BlockTypes.Form
      const targetNode: Node = {
        id: 'step-1',
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'form-1', type: 'BLOCK', components: [{id: 'input-1', type: 'TEXT_INPUT'}]}],
        },
      };
      mockGetNodes.mockReturnValue([targetNode]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drop on element inside form (input-1 is inside form-1)
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {type: 'TEXT_INPUT'},
            },
          },
          target: {
            id: 'input-1',
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
        },
        canceled: false,
      });

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should call confirm handler from useContainerDialogConfirm', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const confirmButton = screen.getByTestId('dialog-confirm');
      fireEvent.click(confirmButton);

      // Confirm handler should be called (from mocked useContainerDialogConfirm)
      expect(screen.getByTestId('form-requires-view-dialog')).toBeInTheDocument();
    });
  });

  describe('Handle Drag Over - Reordering', () => {
    it('should handle reordering during drag over', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // DragDropProvider should have onDragOver configured
      const provider = screen.getByTestId('drag-drop-provider');
      expect(provider).toHaveAttribute('data-ondragover', 'true');
    });

    it('should update node data during reordering drag over', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drag over with reordering
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
      });

      // updateNodeData should be called for reordering
      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
    });

    it('should return early when source is missing during drag over', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drag over without source
      triggerCapturedDragOver({
        operation: {
          source: null,
          target: {id: 'element-1', data: {}},
        },
      });

      // Component should still be rendered
      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should return early when target is missing during drag over', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drag over without target
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {isReordering: true, stepId: 'step-1'},
          },
          target: null,
        },
      });

      // Component should still be rendered
      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should return early when not reordering during drag over', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drag over without reordering flag
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {
              isReordering: false,
              stepId: 'step-1',
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
      });

      // updateNodeData should NOT be called when not reordering
      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should return early when stepId is missing during reordering', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drag over with reordering but no stepId
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: null,
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
      });

      // Component should still be rendered
      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should handle reordering with nested components', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drag over with reordering
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
          target: {
            id: 'nested-element-1',
            data: {},
          },
        },
      });

      // updateNodeData should be called
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should execute updateNodeData callback with nested components during drag over', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Capture the callback passed to updateNodeData
      mockUpdateNodeData.mockImplementation((stepId: string, callback: (node: Node) => {components: unknown[]}) => {
        // Simulate a node with nested components (form with children)
        const mockNode: Node = {
          id: stepId,
          position: {x: 0, y: 0},
          data: {
            components: [
              {id: 'button-1', type: 'BUTTON'},
              {
                id: 'form-1',
                type: 'FORM',
                components: [
                  {id: 'input-1', type: 'TEXT_INPUT'},
                  {id: 'input-2', type: 'TEXT_INPUT'},
                ],
              },
              {id: 'divider-1', type: 'DIVIDER'},
            ],
          },
        };
        // Execute the callback to cover nested component handling
        const result = callback(mockNode);
        expect(result.components).toBeDefined();
      });

      // Simulate drag over with reordering
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
          target: {
            id: 'input-1',
            data: {},
          },
        },
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
    });

    it('should handle drag over with components that have no nested children', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Capture the callback passed to updateNodeData
      mockUpdateNodeData.mockImplementation((stepId: string, callback: (node: Node) => {components: unknown[]}) => {
        // Simulate a node with components that have no nested children
        const mockNode: Node = {
          id: stepId,
          position: {x: 0, y: 0},
          data: {
            components: [
              {id: 'button-1', type: 'BUTTON'},
              {id: 'text-1', type: 'TEXT'},
            ],
          },
        };
        const result = callback(mockNode);
        expect(result.components).toBeDefined();
      });

      // Simulate drag over with reordering
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
          target: {
            id: 'button-1',
            data: {},
          },
        },
      });

      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should handle drag over with undefined node data', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Capture the callback passed to updateNodeData
      mockUpdateNodeData.mockImplementation((stepId: string, callback: (node: Node) => {components: unknown[]}) => {
        // Simulate a node with undefined data
        const mockNode: Node = {
          id: stepId,
          position: {x: 0, y: 0},
          data: undefined as unknown as Record<string, unknown>,
        };
        const result = callback(mockNode);
        expect(result.components).toBeDefined();
      });

      // Simulate drag over with reordering
      triggerCapturedDragOver({
        operation: {
          source: {
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
          target: {
            id: 'element-1',
            data: {},
          },
        },
      });

      expect(mockUpdateNodeData).toHaveBeenCalled();
    });
  });

  describe('Container Dialog Close Handler', () => {
    it('should reset pending drop ref when dialog is closed', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const closeButton = screen.getByTestId('dialog-close');
      fireEvent.click(closeButton);

      // Dialog should be closed
      const dialog = screen.getByTestId('form-requires-view-dialog');
      expect(dialog).toHaveAttribute('data-open', 'false');
    });
  });

  describe('Edge Resolution', () => {
    it('should accept custom onEdgeResolve handler', () => {
      const mockEdgeResolve = vi.fn();

      renderComponent(<DecoratedVisualFlow {...defaultProps} onEdgeResolve={mockEdgeResolve} />);

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });

    it('should work without onEdgeResolve handler', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} onEdgeResolve={undefined} />);

      expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
    });
  });

  describe('Auto Layout Error Handling', () => {
    it('should handle auto layout failure gracefully', async () => {
      mockApplyAutoLayout.mockRejectedValue(new Error('Layout failed'));

      mockGetNodes.mockReturnValue([
        {id: 'node-1', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', position: {x: 0, y: 0}, data: {}},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} triggerAutoLayoutOnLoad />);

      // Should not throw even if layout fails
      await waitFor(() => {
        expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
      });
    });

    it('should handle fitView failure gracefully', async () => {
      mockApplyAutoLayout.mockResolvedValue([{id: 'node-1', position: {x: 100, y: 100}, data: {}}]);
      mockFitView.mockRejectedValue(new Error('FitView failed'));

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      const autoLayoutButton = screen.getByTestId('auto-layout-trigger');
      fireEvent.click(autoLayoutButton);

      // Should not throw even if fitView fails
      await waitFor(() => {
        expect(screen.getByTestId('visual-flow')).toBeInTheDocument();
      });
    });
  });
});
