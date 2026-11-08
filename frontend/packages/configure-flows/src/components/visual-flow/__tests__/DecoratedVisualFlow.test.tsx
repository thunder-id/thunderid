// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-assignment, react/require-default-props */

import type {OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import {render, screen, fireEvent, waitFor, cleanup, act} from '@thunderid/test-utils';
import type {Node, Edge} from '@xyflow/react';
import React from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import FlowVariablesContext from '../../../context/FlowVariablesContext';
import type {Resources} from '../../../models/resources';
import DecoratedVisualFlow from '../DecoratedVisualFlow';

const sharingAccess = {hasChanges: false} as unknown as OrganizationAccessState;

const mockNavigate = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock hooks
vi.mock('../../../hooks/useUIPanelState', () => ({
  default: () => ({
    isResourcePanelOpen: true,
    isResourcePropertiesPanelOpen: false,
    setIsResourcePanelOpen: vi.fn(),
    setIsOpenResourcePropertiesPanel: vi.fn(),
  }),
}));

const {mockFlowConfigState, mockSetFlowEdges} = vi.hoisted(() => ({
  mockFlowConfigState: {isVerboseMode: true},
  mockSetFlowEdges: vi.fn(),
}));

vi.mock('../../../hooks/useFlowConfig', () => ({
  default: () => ({
    isFlowMetadataLoading: false,
    isVerboseMode: mockFlowConfigState.isVerboseMode,
    metadata: undefined,
    setFlowNodes: vi.fn(),
    setFlowEdges: mockSetFlowEdges,
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

const handlerProps = vi.hoisted(() => ({
  current: undefined as {confirmNodesDelete?: (nodes: unknown[]) => Promise<boolean>} | undefined,
}));

vi.mock('../../../hooks/useVisualFlowHandlers', () => ({
  default: (props: {confirmNodesDelete?: (nodes: unknown[]) => Promise<boolean>}) => {
    handlerProps.current = props;

    return {
      handleConnect: vi.fn(),
      handleNodesDelete: vi.fn(),
      handleEdgesDelete: vi.fn(),
    };
  },
}));

const {mockAddToViewAtIndex, mockAddToFormAtIndex} = vi.hoisted(() => ({
  mockAddToViewAtIndex: vi.fn(),
  mockAddToFormAtIndex: vi.fn(),
}));

vi.mock('../../../hooks/useDragDropHandlers', () => ({
  default: () => ({
    addCanvasNode: vi.fn(),
    addToView: vi.fn(),
    addToForm: vi.fn(),
    addToViewAtIndex: mockAddToViewAtIndex,
    addToFormAtIndex: mockAddToFormAtIndex,
  }),
}));

vi.mock('../../../hooks/useContainerDialogConfirm', () => ({
  default: () => vi.fn(),
}));

// Use vi.hoisted for applyAutoLayout mock to ensure it's always available
const {mockApplyAutoLayout} = vi.hoisted(() => ({
  mockApplyAutoLayout: vi.fn().mockResolvedValue([]),
}));

vi.mock('../../../utils/applyAutoLayout', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../utils/applyAutoLayout')>()),
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
const {mockToObject, mockGetNodes, mockGetEdges, mockUpdateNodeData, mockFitView, mockUpdateNodeInternals, mockStore} =
  vi.hoisted(() => ({
    mockToObject: vi.fn(() => ({viewport: {x: 0, y: 0, zoom: 1}})),
    mockGetNodes: vi.fn((): Node[] => []),
    mockGetEdges: vi.fn((): Edge[] => []),
    mockUpdateNodeData: vi.fn(),
    mockFitView: vi.fn().mockResolvedValue(undefined),
    mockUpdateNodeInternals: vi.fn(),
    // The nodes the React Flow store holds, which the flow reads its variables from.
    mockStore: {nodes: [] as Node[]},
  }));

vi.mock('@xyflow/react', () => ({
  Position: {Bottom: 'bottom', Left: 'left', Right: 'right', Top: 'top'},
  useReactFlow: () => ({
    toObject: mockToObject,
    getNodes: mockGetNodes,
    getEdges: mockGetEdges,
    updateNodeData: mockUpdateNodeData,
    fitView: mockFitView,
  }),
  useStore: (selector: (state: {nodes: Node[]; edges: never[]}) => unknown) =>
    selector({edges: [], nodes: mockStore.nodes}),
  useStoreApi: () => ({
    getState: () => ({edges: [], nodeLookup: new Map(), nodes: []}),
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

// Source handed to the DragOverlay render-prop mock below; defaults to undefined so existing
// tests (which don't care about the overlay's own rendering) keep exercising the "nothing being
// dragged" early return.
const dragOverlaySource: {current: unknown} = {current: undefined};

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
    <div data-testid="drag-overlay">
      {typeof children === 'function' ? children(dragOverlaySource.current) : children}
    </div>
  ),
  // Imported by @thunderid/configure-resource-servers, which @thunderid/configure-connections loads.
  useDraggable: () => ({ref: vi.fn(), handleRef: vi.fn(), isDragging: false}),
  useDroppable: () => ({ref: vi.fn(), isDropTarget: false}),
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
// Reads the variables the flow hands to the steps, and offers a button for each way a step can change them.
function VariablesProbe() {
  const {countBindings, ownerOuId, removeBindings, renameBindings, variablesBefore, variablesNodeBefore} =
    React.useContext(FlowVariablesContext);

  return (
    <div data-testid="variables-probe" data-bound-count={countBindings('old')} data-owner={ownerOuId ?? ''}>
      <span
        data-testid="variables-scope"
        data-before={variablesBefore('step-1')
          .map((variable) => variable.identifier)
          .join(',')}
        data-nearest={variablesNodeBefore('step-1') ?? ''}
      />
      <button type="button" data-testid="rename-bindings-trigger" onClick={() => renameBindings('old', 'fresh')}>
        Rename bindings
      </button>
      <button type="button" data-testid="remove-bindings-trigger" onClick={() => removeBindings('old')}>
        Remove bindings
      </button>
    </div>
  );
}

vi.mock('../VisualFlow', () => ({
  default: ({nodes, edges, onNodeDragStop, onNodeClick}: any) => (
    <div
      data-testid="visual-flow"
      data-nodes={JSON.stringify(nodes)}
      data-edges={JSON.stringify(edges)}
      data-has-drag-stop={!!onNodeDragStop}
    >
      <VariablesProbe />
      <button data-testid="node-drag-stop-trigger" onClick={onNodeDragStop}>
        Node Drag Stop
      </button>
      {/* A click on the node body: not a button, so it focuses the node. */}
      <div
        data-testid="node-click-trigger"
        role="presentation"
        onClick={(event) =>
          (onNodeClick as ((e: unknown, n: unknown) => void) | undefined)?.(event, {
            id: 'clicked-node',
            position: {x: 0, y: 0},
          })
        }
      >
        Node Click
      </div>
      {/* A click on a node header action (configure, delete), which reaches
          onNodeClick through the node but must not move the camera. */}
      <button
        data-testid="node-action-click-trigger"
        onClick={(event) =>
          (onNodeClick as ((e: unknown, n: unknown) => void) | undefined)?.(event, {
            id: 'clicked-node',
            position: {x: 0, y: 0},
          })
        }
      >
        Node Action Click
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

vi.mock('../../flow-sharing/useSharedVariables', () => ({
  default: () => new Set<string>(),
}));

vi.mock('../../flow-sharing/FlowSharingDialog', () => ({
  default: ({flow, open, variables}: {flow: {id: string}; open: boolean; variables: unknown[]}) => (
    <div data-testid="flow-sharing-dialog" data-open={open} data-flow={flow.id} data-count={variables.length} />
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

  // The graph after the last update made to the flow's own nodes, which is where bindings are rewritten.
  const afterLastNodesUpdate = (graph: Node[]): Node[] => {
    const updater = defaultProps.setNodes.mock.calls.at(-1)?.[0] as (current: Node[]) => Node[];

    return updater(graph);
  };

  beforeEach(() => {
    vi.clearAllMocks();
    dragOverlaySource.current = undefined;
    mockGetNodes.mockReturnValue([]);
    mockGetEdges.mockReturnValue([]);
    mockStore.nodes = [];
    mockFlowConfigState.isVerboseMode = true;
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

  describe('Deleting a variables step', () => {
    const variablesStep = {
      id: 'vars',
      type: 'VARIABLES',
      position: {x: 0, y: 0},
      data: {inputs: [{identifier: 'region', type: 'TEXT_INPUT'}]},
    } as Node;
    const user = (id: string, properties: Record<string, unknown>): Node =>
      ({id, type: 'TASK_EXECUTION', position: {x: 0, y: 0}, data: {properties}}) as Node;

    const askAboutDeleting = (): Promise<boolean> => {
      let answer: Promise<boolean> | undefined;

      act(() => {
        answer = handlerProps.current?.confirmNodesDelete?.([variablesStep]);
      });

      return answer!;
    };

    it('should delete without asking when nothing uses the variables', async () => {
      mockGetNodes.mockReturnValue([variablesStep, user('step-1', {a: 'plain'})]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      await expect(askAboutDeleting()).resolves.toBe(true);
      expect(screen.queryByTestId('variable-change-dialog')).not.toBeInTheDocument();
    });

    it('should ask, and unbind the steps that use the variables once it is confirmed', async () => {
      mockGetNodes.mockReturnValue([
        variablesStep,
        user('step-1', {a: '{{var(region)}}', keep: 'plain'}),
        user('step-2', {b: '{{var(region)}}'}),
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      const answer = askAboutDeleting();

      expect(await screen.findByTestId('variable-change-dialog')).toHaveTextContent('2 steps use its variables');

      fireEvent.click(screen.getByTestId('variable-change-confirm'));

      await expect(answer).resolves.toBe(true);

      const graph = [
        variablesStep,
        user('step-1', {a: '{{var(region)}}', keep: 'plain'}),
        user('step-2', {b: '{{var(region)}}'}),
      ];

      expect(afterLastNodesUpdate(graph).map((node) => (node.data as {properties?: unknown}).properties)).toEqual([
        undefined,
        {keep: 'plain'},
        {},
      ]);
    });

    it('should keep the step and the bindings when the deletion is declined', async () => {
      mockGetNodes.mockReturnValue([variablesStep, user('step-1', {a: '{{var(region)}}'})]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      const answer = askAboutDeleting();

      fireEvent.click(await screen.findByRole('button', {name: 'Cancel'}));

      await expect(answer).resolves.toBe(false);
      expect(defaultProps.setNodes).not.toHaveBeenCalled();
    });
  });

  describe('Flow variables for the steps', () => {
    const bound = (id: string, properties: Record<string, unknown>): Node =>
      ({id, position: {x: 0, y: 0}, data: {properties}}) as Node;

    it('should tell the steps which organization unit owns the flow, once it can be shared', () => {
      renderComponent(
        <DecoratedVisualFlow
          {...defaultProps}
          sharing={{id: 'flow-1', name: 'Flow', ouId: 'ou-root'}}
          sharingAccess={sharingAccess}
        />,
      );

      expect(screen.getByTestId('variables-probe')).toHaveAttribute('data-owner', 'ou-root');
    });

    it('should leave the owner out for a flow that is not saved yet', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('variables-probe')).toHaveAttribute('data-owner', '');
    });

    it('should offer a step only the variables every route passes before it', () => {
      const variables = (id: string, identifier: string): Node =>
        ({id, type: 'VARIABLES', position: {x: 0, y: 0}, data: {inputs: [{identifier, type: 'TEXT_INPUT'}]}}) as Node;
      const graph = [
        {id: 'start', type: 'START', position: {x: 0, y: 0}, data: {}} as Node,
        variables('vars-1', 'before'),
        {id: 'step-1', type: 'TASK_EXECUTION', position: {x: 0, y: 0}, data: {}} as Node,
        variables('vars-2', 'beside'),
      ];

      mockStore.nodes = graph;
      mockGetNodes.mockReturnValue(graph);
      mockGetEdges.mockReturnValue([
        {id: 'e1', source: 'start', target: 'vars-1'},
        {id: 'e2', source: 'vars-1', target: 'step-1'},
        {id: 'e3', source: 'start', target: 'vars-2'},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('variables-scope')).toHaveAttribute('data-before', 'before');
      expect(screen.getByTestId('variables-scope')).toHaveAttribute('data-nearest', 'vars-1');
    });

    it('should offer nothing to a step a variables node can be bypassed to reach', () => {
      const graph = [
        {id: 'start', type: 'START', position: {x: 0, y: 0}, data: {}} as Node,
        {
          id: 'vars-1',
          type: 'VARIABLES',
          position: {x: 0, y: 0},
          data: {inputs: [{identifier: 'a', type: 'TEXT_INPUT'}]},
        } as Node,
        {id: 'step-1', type: 'TASK_EXECUTION', position: {x: 0, y: 0}, data: {}} as Node,
      ];

      mockStore.nodes = graph;
      mockGetNodes.mockReturnValue(graph);
      mockGetEdges.mockReturnValue([
        {id: 'e1', source: 'start', target: 'vars-1'},
        {id: 'e2', source: 'vars-1', target: 'step-1'},
        {id: 'e3', source: 'start', target: 'step-1'},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('variables-scope')).toHaveAttribute('data-before', '');
      expect(screen.getByTestId('variables-scope')).toHaveAttribute('data-nearest', '');
    });

    it('should count the steps that hold a property bound to a variable', () => {
      mockGetNodes.mockReturnValue([
        bound('step-1', {a: '{{var(old)}}'}),
        bound('step-2', {a: '{{var(other)}}'}),
        bound('step-3', {a: '{{var(old)}}', b: '{{var(old)}}'}),
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('variables-probe')).toHaveAttribute('data-bound-count', '2');
    });

    it('should point the bound properties of every step at a renamed variable', () => {
      mockGetNodes.mockReturnValue([
        bound('step-1', {a: '{{var(old)}}', keep: 'plain'}),
        bound('step-2', {a: '{{var(other)}}'}),
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      fireEvent.click(screen.getByTestId('rename-bindings-trigger'));

      const graph = [bound('step-1', {a: '{{var(old)}}', keep: 'plain'}), bound('step-2', {a: '{{var(other)}}'})];
      const result = afterLastNodesUpdate(graph);

      expect((result[0].data as {properties: unknown}).properties).toEqual({a: '{{var(fresh)}}', keep: 'plain'});
      // A step bound to another variable is the very same node, untouched.
      expect(result[1]).toBe(graph[1]);
    });

    it('should rewrite the steps a compact stack hides, which the canvas cannot reach', () => {
      const hidden = bound('hidden-member', {a: '{{var(old)}}'});
      const shown = bound('shown', {a: '{{var(old)}}'});

      // The canvas shows only one of the two steps, as it does with a stack collapsed.
      mockGetNodes.mockReturnValue([shown]);
      mockStore.nodes = [shown];

      renderComponent(<DecoratedVisualFlow {...defaultProps} nodes={[shown]} sourceNodes={[shown, hidden]} />);

      expect(screen.getByTestId('variables-probe')).toHaveAttribute('data-bound-count', '2');

      fireEvent.click(screen.getByTestId('rename-bindings-trigger'));

      const result = afterLastNodesUpdate([shown, hidden]);

      expect(result.map((node) => (node.data as {properties: unknown}).properties)).toEqual([
        {a: '{{var(fresh)}}'},
        {a: '{{var(fresh)}}'},
      ]);
      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });

    it('should take the bound properties off a removed variable', () => {
      mockGetNodes.mockReturnValue([bound('step-1', {a: '{{var(old)}}', keep: 'plain'})]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      fireEvent.click(screen.getByTestId('remove-bindings-trigger'));

      const result = afterLastNodesUpdate([bound('step-1', {a: '{{var(old)}}', keep: 'plain'})]);

      expect((result[0].data as {properties: unknown}).properties).toEqual({keep: 'plain'});
    });
  });

  describe('Save blocked by the sharing', () => {
    const sharing = {id: 'flow-1', name: 'Enterprise sign-in', ouId: 'ou-root'};

    it('should disable Save while a reason blocks it, and mark the Share button', () => {
      renderComponent(
        <DecoratedVisualFlow
          {...defaultProps}
          onSave={vi.fn()}
          sharing={sharing}
          sharingAccess={sharingAccess}
          saveBlockedReason="Fix the defaults first"
        />,
      );

      expect(screen.getByTestId('save-flow-button')).toBeDisabled();
      expect(screen.getByTestId('share-flow-button')).toHaveClass('MuiButton-colorError');
    });

    it('should leave Save enabled, and Share unmarked, when nothing blocks it', () => {
      renderComponent(
        <DecoratedVisualFlow {...defaultProps} onSave={vi.fn()} sharing={sharing} sharingAccess={sharingAccess} />,
      );

      expect(screen.getByTestId('save-flow-button')).toBeEnabled();
      expect(screen.getByTestId('share-flow-button')).not.toHaveClass('MuiButton-colorError');
    });
  });

  describe('Share button', () => {
    const sharing = {id: 'flow-1', name: 'Enterprise sign-in', ouId: 'ou-root'};
    const variablesNode = {
      id: 'variables-1',
      type: 'VARIABLES',
      position: {x: 0, y: 0},
      data: {inputs: [{identifier: 'connection1'}]},
    } as Node;

    it('should not render for a flow that cannot be shared yet', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.queryByTestId('share-flow-button')).not.toBeInTheDocument();
      expect(screen.queryByTestId('flow-sharing-dialog')).not.toBeInTheDocument();
    });

    it('should render beside the other header buttons for a flow that can be shared', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} sharing={sharing} sharingAccess={sharingAccess} />);

      expect(screen.getByTestId('share-flow-button')).toBeInTheDocument();
    });

    it('should open the sharing dialog for the flow, with its variables', () => {
      mockStore.nodes = [variablesNode];
      renderComponent(
        <DecoratedVisualFlow
          {...defaultProps}
          nodes={[variablesNode]}
          sharing={sharing}
          sharingAccess={sharingAccess}
        />,
      );

      expect(screen.getByTestId('flow-sharing-dialog')).toHaveAttribute('data-open', 'false');

      fireEvent.click(screen.getByTestId('share-flow-button'));

      const dialog = screen.getByTestId('flow-sharing-dialog');

      expect(dialog).toHaveAttribute('data-open', 'true');
      expect(dialog).toHaveAttribute('data-flow', 'flow-1');
      expect(dialog).toHaveAttribute('data-count', '1');
    });

    it('should share a flow with no variables node with no variables', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} sharing={sharing} sharingAccess={sharingAccess} />);

      expect(screen.getByTestId('flow-sharing-dialog')).toHaveAttribute('data-count', '0');
    });
  });

  describe('Variables node', () => {
    const variablesNode = {
      id: 'variables-1',
      type: 'VARIABLES',
      position: {x: 0, y: 0},
      data: {inputs: [{identifier: 'connection'}, {identifier: 'support_email'}]},
    } as Node;

    it('should share the flow with the variables of every variables node', () => {
      const secondNode = {
        id: 'variables-2',
        type: 'VARIABLES',
        position: {x: 0, y: 200},
        data: {inputs: [{identifier: 'phone'}]},
      } as Node;

      mockStore.nodes = [variablesNode, secondNode];

      renderComponent(
        <DecoratedVisualFlow
          {...defaultProps}
          nodes={[variablesNode, secondNode]}
          sharing={{id: 'flow-1', name: 'Flow', ouId: 'ou-root'}}
          sharingAccess={sharingAccess}
        />,
      );

      expect(screen.getByTestId('flow-sharing-dialog')).toHaveAttribute('data-count', '3');
    });
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

    it('should keep save enabled while previewing (clicking it stops the preview first)', () => {
      const nodes = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}] as Node[];
      renderComponent(<DecoratedVisualFlow {...defaultProps} nodes={nodes} onSave={vi.fn()} />);

      expect(screen.getByTestId('save-flow-button')).toBeEnabled();

      fireEvent.click(screen.getByTestId('simulate-flow-button'));

      expect(screen.getByTestId('save-flow-button')).toBeEnabled();
    });

    it('should focus the clicked node via fitView', () => {
      // Focusing is skipped for a node that is no longer on the canvas.
      mockGetNodes.mockReturnValue([{id: 'clicked-node', position: {x: 0, y: 0}, data: {}}]);

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

  describe('Navigation', () => {
    it('should navigate to the flows list when the back button is clicked', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      fireEvent.click(screen.getByRole('button', {name: /back/i}));

      expect(mockNavigate).toHaveBeenCalledWith('/flows');
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

      expect(screen.getByTestId('save-flow-button')).toBeInTheDocument();
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

      const saveButton = screen.getByTestId('save-flow-button');
      fireEvent.click(saveButton);

      expect(mockOnSave).toHaveBeenCalledWith({
        nodes: [{id: 'node-1', position: {x: 0, y: 0}, data: {}}],
        edges: [{id: 'edge-1', source: 'node-1', target: 'node-2'}],
        viewport: {x: 10, y: 20, zoom: 1.5},
      });
    });

    it('should not throw when onSave is not provided', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} onSave={undefined} />);

      const saveButton = screen.getByTestId('save-flow-button');
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

    it('should use detailed spacing and measured sizes in verbose mode', async () => {
      mockGetNodes.mockReturnValue([
        {id: 'exec-1', type: 'TASK_EXECUTION', position: {x: 0, y: 0}, data: {}, measured: {width: 350, height: 140}},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      fireEvent.click(screen.getByTestId('auto-layout-trigger'));

      await waitFor(() => {
        expect(mockApplyAutoLayout).toHaveBeenCalled();
      });
      const [layoutNodes, , options] = mockApplyAutoLayout.mock.calls[0] as [Node[], Edge[], Record<string, number>];
      expect(layoutNodes[0].measured).toEqual({width: 350, height: 140});
      expect(options).toMatchObject({nodeSpacing: 100, rankSpacing: 160});
    });

    it('should use tight spacing and chip sizes in compact mode', async () => {
      mockFlowConfigState.isVerboseMode = false;
      mockGetNodes.mockReturnValue([
        {id: 'exec-1', type: 'TASK_EXECUTION', position: {x: 0, y: 0}, data: {}, measured: {width: 350, height: 140}},
        {
          id: 'stack-1',
          type: 'EXECUTION_STACK',
          position: {x: 0, y: 0},
          data: {memberIds: ['a', 'b', 'c'], members: []},
        },
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      fireEvent.click(screen.getByTestId('auto-layout-trigger'));

      await waitFor(() => {
        expect(mockApplyAutoLayout).toHaveBeenCalled();
      });
      const [layoutNodes, , options] = mockApplyAutoLayout.mock.calls[0] as [Node[], Edge[], Record<string, number>];
      expect(layoutNodes[0].measured).toEqual({width: 48, height: 48});
      // 3 members: 48 chip + 2 deck-layer offsets of 4
      expect(layoutNodes[1].measured).toEqual({width: 56, height: 48});
      expect(options).toMatchObject({nodeSpacing: 60, rankSpacing: 80});
    });

    it('should map stack positions back onto member nodes', async () => {
      const setNodes = vi.fn();
      mockGetNodes.mockReturnValue([
        {id: 'stack-1', type: 'EXECUTION_STACK', position: {x: 0, y: 0}, data: {memberIds: ['a', 'b'], members: []}},
      ]);
      mockApplyAutoLayout.mockResolvedValue([
        {id: 'stack-1', position: {x: 200, y: 300}, data: {memberIds: ['a', 'b'], members: []}},
      ]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} setNodes={setNodes} />);
      fireEvent.click(screen.getByTestId('auto-layout-trigger'));

      await waitFor(() => {
        expect(setNodes).toHaveBeenCalled();
      });
      const updater = setNodes.mock.calls[0][0] as (nodes: Node[]) => Node[];
      const updated = updater([
        {id: 'a', position: {x: 1, y: 1}, data: {}},
        {id: 'b', position: {x: 2, y: 2}, data: {}},
        {id: 'other', position: {x: 3, y: 3}, data: {}},
      ]);
      expect(updated[0].position).toEqual({x: 200, y: 300});
      expect(updated[1].position).toEqual({x: 200, y: 300});
      expect(updated[2].position).toEqual({x: 3, y: 3});
    });
  });

  describe('Validation graph sync', () => {
    it('should publish the source edges rather than the compact display edges', () => {
      const displayEdges = [
        {id: 'e1', source: 'view-1', sourceHandle: 'button-1_NEXT', target: 'execution-stack_exec-a'},
      ] as Edge[];
      const sourceEdges = [{id: 'e1', source: 'view-1', sourceHandle: 'button-1_NEXT', target: 'exec-a'}] as Edge[];

      renderComponent(<DecoratedVisualFlow {...defaultProps} edges={displayEdges} sourceEdges={sourceEdges} />);

      // A stack rewires an edge onto a synthetic id, which would hide the
      // element's real target from graph validation rules.
      expect(mockSetFlowEdges).toHaveBeenCalledWith(sourceEdges);
    });

    it('should publish the edges as-is when there is no display transform', () => {
      const edges = [{id: 'e1', source: 'view-1', sourceHandle: 'button-1_NEXT', target: 'exec-a'}] as Edge[];

      renderComponent(<DecoratedVisualFlow {...defaultProps} edges={edges} />);

      expect(mockSetFlowEdges).toHaveBeenCalledWith(edges);
    });
  });

  describe('Compact Mode Toggle', () => {
    it('should run the shared auto layout when switching to compact mode', async () => {
      const {rerender} = renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      expect(mockApplyAutoLayout).not.toHaveBeenCalled();

      mockFlowConfigState.isVerboseMode = false;
      rerender(<DecoratedVisualFlow {...defaultProps} />);

      await waitFor(() => {
        expect(mockApplyAutoLayout).toHaveBeenCalled();
      });
    });

    it('should run the shared auto layout when switching back to detailed mode', async () => {
      mockFlowConfigState.isVerboseMode = false;
      const {rerender} = renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      expect(mockApplyAutoLayout).not.toHaveBeenCalled();

      mockFlowConfigState.isVerboseMode = true;
      rerender(<DecoratedVisualFlow {...defaultProps} />);

      await waitFor(() => {
        expect(mockApplyAutoLayout).toHaveBeenCalled();
      });
    });
  });

  describe('Node focus on click', () => {
    it('should focus a node that is still on the canvas', () => {
      mockGetNodes.mockReturnValue([{id: 'clicked-node', position: {x: 0, y: 0}, data: {}}]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      screen.getByTestId('node-click-trigger').click();

      expect(mockFitView).toHaveBeenCalledWith({
        nodes: [{id: 'clicked-node'}],
        padding: 0.3,
        maxZoom: 1.2,
        duration: 500,
      });
    });

    it('should leave the viewport alone when a node header action is clicked', () => {
      // Deleting from the node header queues a fitView that resolves after the
      // node is gone, which fits nothing and snaps the canvas to the origin.
      mockGetNodes.mockReturnValue([{id: 'clicked-node', position: {x: 0, y: 0}, data: {}}]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      screen.getByTestId('node-action-click-trigger').click();

      expect(mockFitView).not.toHaveBeenCalled();
    });

    it('should leave the viewport alone when the clicked node is already gone', () => {
      // Deleting a node clicks it on the way out, and React Flow can deliver
      // that click after the removal. Fitting to it would match nothing and
      // snap the canvas to the origin.
      mockGetNodes.mockReturnValue([{id: 'some-other-node', position: {x: 0, y: 0}, data: {}}]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);
      screen.getByTestId('node-click-trigger').click();

      expect(mockFitView).not.toHaveBeenCalled();
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

    it('should handle a SELECT drop on canvas as an input-on-canvas scenario', async () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {category: 'FIELD', type: 'SELECT'},
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

    it('should handle drop on element inside a stack nested inside a form', () => {
      // Form contains a Stack, and the Stack contains the drop target element.
      const targetNode: Node = {
        id: 'step-1',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: 'BLOCK',
              components: [
                {
                  id: 'stack-1',
                  type: 'STACK',
                  components: [{id: 'button-1', type: 'BUTTON'}],
                },
              ],
            },
          ],
        },
      };
      mockGetNodes.mockReturnValue([targetNode]);

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      // Simulate drop on button-1, which lives inside stack-1, which lives inside form-1.
      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {type: 'IMAGE'},
            },
          },
          target: {
            id: 'button-1',
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
        },
        canceled: false,
      });

      // The nested stack (not the form, and not a view-level fallback) should be the target container.
      expect(mockAddToFormAtIndex).toHaveBeenCalledWith(expect.anything(), 'step-1', 'stack-1', 'button-1');
      expect(mockAddToViewAtIndex).not.toHaveBeenCalled();
    });

    it('should not insert at a specific position when the reordering drop target has no string id', () => {
      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      triggerCapturedDragEnd({
        operation: {
          source: {
            data: {
              dragged: {type: 'IMAGE'},
            },
          },
          target: {
            data: {
              isReordering: true,
              stepId: 'step-1',
            },
          },
        },
        canceled: false,
      });

      expect(mockAddToFormAtIndex).not.toHaveBeenCalled();
      expect(mockAddToViewAtIndex).not.toHaveBeenCalled();
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

  describe('Drag Overlay', () => {
    it('should render nothing when nothing is being dragged', () => {
      dragOverlaySource.current = undefined;

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('drag-overlay')).toBeEmptyDOMElement();
    });

    it('should render nothing when the dragged source has no resource', () => {
      dragOverlaySource.current = {data: {isReordering: true}};

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByTestId('drag-overlay')).toBeEmptyDOMElement();
    });

    it('should show the resource display label when present', () => {
      dragOverlaySource.current = {
        data: {isReordering: true, resource: {type: 'TEXT_INPUT', display: {label: 'Name'}}},
      };

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByText('Name')).toBeInTheDocument();
    });

    it('should fall back to the resource type when there is no display label', () => {
      dragOverlaySource.current = {
        data: {isReordering: true, resource: {type: 'TEXT_INPUT'}},
      };

      renderComponent(<DecoratedVisualFlow {...defaultProps} />);

      expect(screen.getByText('TEXT_INPUT')).toBeInTheDocument();
    });
  });
});
