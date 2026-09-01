// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CollisionPriority} from '@dnd-kit/abstract';
import {move} from '@dnd-kit/helpers';
import {DragDropProvider, DragOverlay, type DragDropEventHandlers} from '@dnd-kit/react';
import {useIdentityProviders, useSMSProviders} from '@thunderid/configure-connections';
import {Badge, Box, Button, Card, CardContent, Tooltip, Typography, type Theme} from '@wso2/oxygen-ui';
import {ArrowLeft, Play, Save, Square} from '@wso2/oxygen-ui-icons-react';
import {
  type Connection,
  type Edge,
  type Node,
  type OnEdgesChange,
  type OnNodesChange,
  useReactFlow,
  useUpdateNodeInternals,
} from '@xyflow/react';
import type {UpdateNodeInternals} from '@xyflow/system';
import cloneDeep from 'lodash-es/cloneDeep';
import {
  type Dispatch,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
  type ReactElement,
  type ReactNode,
  type SetStateAction,
} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import CanvasContextMenu, {type CanvasContextMenuTarget} from './CanvasContextMenu';
import CanvasToolbar from './CanvasToolbar';
import DiscardChangesDialog from './DiscardChangesDialog';
import FormRequiresViewDialog from './FormRequiresViewDialog';
import SimulationStepPreview from './SimulationStepPreview';
import ValidationBadge from './ValidationBadge';
import VisualFlow, {type VisualFlowPropsInterface} from './VisualFlow';
import VisualFlowConstants from '../../constants/VisualFlowConstants';
import StepPreviewContext from '../../context/StepPreviewContext';
import useComponentDelete from '../../hooks/useComponentDelete';
import useConfirmPasswordField from '../../hooks/useConfirmPasswordField';
import useContainerDialogConfirm from '../../hooks/useContainerDialogConfirm';
import useDeleteExecutionResource from '../../hooks/useDeleteExecutionResource';
import useDragDropHandlers from '../../hooks/useDragDropHandlers';
import useFlowConfig from '../../hooks/useFlowConfig';
import useFlowEvents from '../../hooks/useFlowEvents';
import useFlowRoutes from '../../hooks/useFlowRoutes';
import useFlowSimulation from '../../hooks/useFlowSimulation';
import useGenerateStepElement from '../../hooks/useGenerateStepElement';
import useInteractionState from '../../hooks/useInteractionState';
import useResourceAdd from '../../hooks/useResourceAdd';
import useStaticContentField from '../../hooks/useStaticContentField';
import useUIPanelState from '../../hooks/useUIPanelState';
import useValidationStatus from '../../hooks/useValidationStatus';
import useVisualFlowHandlers from '../../hooks/useVisualFlowHandlers';
import type {DragSourceData, DragTargetData, DragEventWithNative} from '../../models/drag-drop';
import {BlockTypes, type Element} from '../../models/elements';
import type {MetadataInterface} from '../../models/metadata';
import Notification, {NotificationType} from '../../models/notification';
import {ResourceTypes, type Resource, type Resources} from '../../models/resources';
import {StepTypes, type Step, type StepData} from '../../models/steps';
import {type Template} from '../../models/templates';
import type {Widget} from '../../models/widget';
import applyAutoLayout, {hasUnpositionedNodes} from '../../utils/applyAutoLayout';
import buildStepPropertiesResource from '../../utils/buildStepPropertiesResource';
import {EXECUTION_STACK_NODE_TYPE, getExecutionStackWidth} from '../../utils/compactGraphTransforms';
import computeExecutorConnections from '../../utils/computeExecutorConnections';
import {canDuplicateNode, duplicateFlowNode} from '../../utils/duplicateFlowNode';
import generateResourceId from '../../utils/generateResourceId';
import {resolveCollisions} from '../../utils/resolveCollisions';
import {
  stripSimulationEdgeClasses,
  stripSimulationNodeClasses,
  withSimulationClasses,
} from '../../utils/stripSimulationClasses';
import {findContainingComponent} from '../../utils/updateNestedComponent';
import {widgetNeedsViewContainer} from '../../utils/widgetUtils';
import Droppable from '../dnd/Droppable';
import EdgePathsProvider from '../react-flow-overrides/EdgePathsProvider';
import ResourcePanel from '../resource-panel/ResourcePanel';
import ResourcePropertyPanel from '../resource-property-panel/ResourcePropertyPanel';
import ValidationPanel from '../validation-panel/ValidationPanel';

/**
 * Props interface of {@link DecoratedVisualFlow}
 */
export interface DecoratedVisualFlowPropsInterface extends Omit<VisualFlowPropsInterface, 'edgeTypes'> {
  resources: Resources;
  edgeTypes?: VisualFlowPropsInterface['edgeTypes'];
  onEdgeResolve?: (connection: Connection, nodes: Node[]) => Edge;
  initialNodes?: Node[];
  initialEdges?: Edge[];
  nodes: Node[];
  edges: Edge[];
  /**
   * The untransformed graph nodes when `nodes` carries a display-only
   * transform (compact-mode executor stacks). Used for validation syncing and
   * persistence so hidden stack members are never lost. Defaults to `nodes`.
   */
  sourceNodes?: Node[];
  /**
   * The untransformed graph edges accompanying `sourceNodes`. Defaults to
   * `edges`.
   */
  sourceEdges?: Edge[];
  mutateComponents: (components: Element[]) => Element[];
  onTemplateLoad: (template: Template) => [Node[], Edge[], Resource?, string?];
  onWidgetLoad: (
    widget: Widget,
    targetResource: Resource,
    currentNodes: Node[],
    edges: Edge[],
  ) => [Node[], Edge[], Resource | null, string | null];
  onStepLoad: (step: Step) => Step;
  onResourceAdd: (resource: Resource) => void;
  setNodes: Dispatch<SetStateAction<Node[]>>;
  setEdges: Dispatch<SetStateAction<Edge[]>>;
  onNodesChange: OnNodesChange<Node>;
  onEdgesChange: OnEdgesChange<Edge>;
  flowTitle: string;
  flowHandle: string;
  onFlowTitleChange: (newTitle: string) => void;
  onSave?: (canvasData: {nodes: Node[]; edges: Edge[]; viewport: {x: number; y: number; zoom: number}}) => void;
  /**
   * When true, triggers auto-layout on initial render if nodes lack proper layout data.
   * This is useful when loading flows that don't have saved canvas positions.
   */
  triggerAutoLayoutOnLoad?: boolean;
  /**
   * Extra controls rendered at the bottom of the resource panel, below the resource sections.
   */
  resourcePanelFooter?: ReactNode;
  /**
   * Undo the last canvas edit. When provided, undo/redo controls appear in the toolbar.
   */
  onUndo?: () => void;
  /**
   * Redo the last undone canvas edit.
   */
  onRedo?: () => void;
  /**
   * Whether an undo step is available.
   */
  canUndo?: boolean;
  /**
   * Whether a redo step is available.
   */
  canRedo?: boolean;
  /**
   * Whether the flow has unsaved changes (drives the Save-button indicator).
   */
  isDirty?: boolean;
}

/**
 * Decorated visual flow component with drag-and-drop support.
 *
 * @param props - Props injected to the component.
 * @returns The DecoratedVisualFlow component.
 */
function DecoratedVisualFlow({
  resources,
  nodes,
  edges,
  sourceNodes = undefined,
  sourceEdges = undefined,
  setNodes,
  setEdges,
  onNodesChange,
  onEdgesChange,
  onEdgeResolve = undefined,
  edgeTypes = {},
  mutateComponents,
  onTemplateLoad,
  onWidgetLoad,
  onStepLoad,
  onSave = undefined,
  flowTitle,
  flowHandle,
  onFlowTitleChange,
  triggerAutoLayoutOnLoad = false,
  resourcePanelFooter = undefined,
  onUndo = undefined,
  onRedo = undefined,
  canUndo = false,
  canRedo = false,
  isDirty = false,
  ...rest
}: DecoratedVisualFlowPropsInterface): ReactElement {
  useDeleteExecutionResource();
  useConfirmPasswordField();
  useStaticContentField();

  const {toObject, getNodes, getEdges, updateNodeData, fitView, deleteElements, screenToFlowPosition} = useReactFlow();
  const updateNodeInternals: UpdateNodeInternals = useUpdateNodeInternals();
  const {deleteComponent} = useComponentDelete();
  const {isResourcePanelOpen, isResourcePropertiesPanelOpen, setIsResourcePanelOpen, setIsOpenResourcePropertiesPanel} =
    useUIPanelState();
  const {notifyElementAdded, onAutoLayout} = useFlowEvents();
  const {
    isFlowMetadataLoading,
    isMiniMapVisible,
    isSnapToGridEnabled,
    isVerboseMode,
    metadata,
    setFlowNodes,
    setFlowEdges,
  } = useFlowConfig();
  const {onResourceDropOnCanvas, setLastInteractedResource, setLastInteractedStepId} = useInteractionState();

  // Sync controlled nodes to the shared FlowConfig context so that
  // ValidationProvider (which sits above this ReactFlowProvider) can
  // compute validation notifications from the current node data.
  // Only sync when node data actually changes — skip position-only
  // updates (drag) to avoid unnecessary validation recomputation.
  // Track data references instead of JSON.stringify to avoid O(n) serialization per render.
  const prevNodeDataRefsRef = useRef<Map<string, unknown>>(new Map());

  useEffect(() => {
    // Validation must see the real graph, not the compact display transform
    // (which hides stacked executor members).
    const validationNodes = sourceNodes ?? nodes;
    let dataChanged = validationNodes.length !== prevNodeDataRefsRef.current.size;

    if (!dataChanged) {
      for (const node of validationNodes) {
        if (prevNodeDataRefsRef.current.get(node.id) !== node.data) {
          dataChanged = true;
          break;
        }
      }
    }

    if (dataChanged) {
      const newRefs = new Map<string, unknown>();
      for (const node of validationNodes) {
        newRefs.set(node.id, node.data);
      }
      prevNodeDataRefsRef.current = newRefs;
      setFlowNodes(validationNodes);
    }
  }, [nodes, sourceNodes, setFlowNodes]);

  // Edges are pushed alongside the nodes so graph validation rules can inspect
  // what an element connects to. Like the nodes, these must be the real graph
  // rather than the compact display transform, whose stack rewiring would hide
  // an element's true target. Keyed on a structural signature rather than array
  // identity, which also changes on hover and simulation decoration.
  const validationEdges = sourceEdges ?? edges;
  const edgeSignature = validationEdges
    .map((edge) => `${edge.id}|${edge.source}|${edge.sourceHandle ?? ''}|${edge.target}`)
    .join(';');

  useEffect(() => {
    setFlowEdges(validationEdges);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [edgeSignature, setFlowEdges]);
  const {generateStepElement} = useGenerateStepElement();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const flowRoutes = useFlowRoutes();
  const {notifications, openValidationPanel} = useValidationStatus();

  const {errorCount, warningCount} = useMemo(() => {
    let errors = 0;
    let warnings = 0;

    notifications?.forEach((notification: Notification) => {
      const type = notification.getType();
      if (type === NotificationType.ERROR) errors += 1;
      else if (type === NotificationType.WARNING) warnings += 1;
    });

    return {errorCount: errors, warningCount: warnings};
  }, [notifications]);

  const hasErrors = errorCount > 0;

  // Fetch identity providers and SMS providers to compute executor connections
  const {data: identityProviders} = useIdentityProviders();
  const {data: smsProviders} = useSMSProviders();
  const computedMetadata: MetadataInterface | undefined = useMemo(() => {
    const executorConnections = computeExecutorConnections({identityProviders, smsProviders});

    if (executorConnections.length === 0 && !metadata) {
      return undefined;
    }

    return {
      ...metadata,
      executorConnections: executorConnections.length > 0 ? executorConnections : (metadata?.executorConnections ?? []),
    } as MetadataInterface;
  }, [identityProviders, smsProviders, metadata]);

  const [isContainerDialogOpen, setIsContainerDialogOpen] = useState<boolean>(false);
  const [dropScenario, setDropScenario] = useState<
    'form-on-canvas' | 'input-on-canvas' | 'input-on-view' | 'widget-on-canvas'
  >('form-on-canvas');

  const pendingDropRef = useRef<{
    event: DragEventWithNative;
    sourceData: DragSourceData;
    targetData: DragTargetData;
  } | null>(null);

  const handleContainerDialogClose = useCallback((): void => {
    setIsContainerDialogOpen(false);
    pendingDropRef.current = null;
  }, []);

  const handleContainerDialogConfirm = useContainerDialogConfirm({
    dropScenario,
    handleContainerDialogClose,
    generateStepElement,
    onStepLoad,
    setNodes,
    setEdges,
    onResourceDropOnCanvas,
    onWidgetLoad,
    metadata: computedMetadata,
    pendingDropRef,
  });

  const handleOnAdd = useResourceAdd({
    onTemplateLoad,
    onWidgetLoad,
    onStepLoad,
    setNodes,
    setEdges,
    generateStepElement,
    metadata: computedMetadata,
    onResourceDropOnCanvas,
  });

  const {handleConnect, handleNodesDelete, handleEdgesDelete} = useVisualFlowHandlers({
    onEdgeResolve,
    setEdges,
  });

  const {addCanvasNode, addToView, addToForm, addToViewAtIndex, addToFormAtIndex} = useDragDropHandlers({
    onStepLoad,
    setNodes,
    setEdges,
    onResourceDropOnCanvas,
    generateStepElement,
    mutateComponents,
    onWidgetLoad,
    metadata: computedMetadata,
  });

  // Memoized handleSave. Nodes/edges come back from the React Flow store, which
  // holds the decorated display arrays while previewing — strip the simulation
  // styling so it is never persisted into the flow's layout data.
  const persistCanvas = useCallback((): void => {
    const {viewport} = toObject();
    // Persist the real graph: in compact mode the canvas holds a display-only
    // transform (executor stacks) that must never reach the flow definition.
    const canvasData = {
      nodes: sourceNodes ?? stripSimulationNodeClasses(getNodes()),
      edges: sourceEdges ?? stripSimulationEdgeClasses(getEdges()),
      viewport,
    };
    onSave?.(canvasData);
  }, [toObject, getNodes, getEdges, onSave, sourceNodes, sourceEdges]);

  // The one auto-layout path for both the toolbar button and the compact
  // toggle. It is mode-aware: compact mode uses tighter spacing and feeds the
  // layout engine the known chip/stack sizes (which may not be measured yet
  // right after a toggle), so re-running it in compact mode reproduces the
  // same tight layout instead of rearranging with detailed-mode metrics.
  const handleAutoLayout = useCallback((): void => {
    const currentNodes = stripSimulationNodeClasses(getNodes());
    const currentEdges = getEdges();

    const chipSize = VisualFlowConstants.FLOW_BUILDER_COMPACT_EXECUTION_NODE_SIZE;
    const layoutNodes = isVerboseMode
      ? currentNodes
      : currentNodes.map((node) => {
          if (node.type === StepTypes.Execution) {
            return {...node, measured: {height: chipSize, width: chipSize}};
          }
          if (node.type === EXECUTION_STACK_NODE_TYPE) {
            const memberCount = (node.data as {memberIds?: string[]} | undefined)?.memberIds?.length ?? 1;
            return {...node, measured: {height: chipSize, width: getExecutionStackWidth(memberCount)}};
          }
          return node;
        });

    const spacing = isVerboseMode ? {nodeSpacing: 100, rankSpacing: 160} : {nodeSpacing: 60, rankSpacing: 80};

    applyAutoLayout(layoutNodes, currentEdges, {...spacing, offsetX: 50, offsetY: 50})
      .then((layoutedNodes) => {
        // Map positions back onto the graph state by id. Synthetic stack
        // nodes expand onto their members so the state graph follows the
        // display layout without ever absorbing display-only nodes.
        const positionById = new Map<string, {x: number; y: number}>();
        layoutedNodes.forEach((node) => {
          positionById.set(node.id, node.position);
          const memberIds = (node.data as {memberIds?: string[]} | undefined)?.memberIds;
          memberIds?.forEach((memberId) => positionById.set(memberId, node.position));
        });
        setNodes((nodesNow) =>
          nodesNow.map((node) => {
            const position = positionById.get(node.id);
            return position ? {...node, position} : node;
          }),
        );
        requestAnimationFrame(() => {
          fitView({padding: 0.2, duration: 300}).catch(() => {
            // Ignore fitView errors - layout is still applied
          });
        });
      })
      .catch(() => {
        // Layout failed, keep original positions
      });
  }, [getNodes, getEdges, setNodes, fitView, isVerboseMode]);

  // Every compact/detailed toggle re-runs the shared auto-layout path above
  // so the canvas always fits the size the nodes render at in the new mode.
  // Two frames are awaited so the swapped node components are committed and
  // re-measured before the layout reads their sizes (compact feeds explicit
  // chip sizes; detailed relies on the fresh measurements).
  const prevVerboseModeRef = useRef<boolean>(isVerboseMode);

  useEffect(() => {
    if (prevVerboseModeRef.current === isVerboseMode) {
      return;
    }
    prevVerboseModeRef.current = isVerboseMode;

    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        handleAutoLayout();
      });
    });
  }, [isVerboseMode, handleAutoLayout]);

  // Track whether auto-layout has been triggered to prevent multiple triggers
  const autoLayoutTriggeredRef = useRef<boolean>(false);

  // Listen for auto-layout trigger events from parent components
  useEffect(() => onAutoLayout(handleAutoLayout), [onAutoLayout, handleAutoLayout]);

  // Effect to trigger auto-layout on initial load when nodes lack proper layout data
  useEffect(() => {
    if (!triggerAutoLayoutOnLoad || autoLayoutTriggeredRef.current) {
      return;
    }

    const currentNodes = getNodes();

    // Skip if no nodes or only one node (nothing to layout)
    if (currentNodes.length <= 1) {
      return;
    }

    // Nodes need a layout when several sit at the origin, i.e. the flow was
    // stored without layout data. FlowBuilder uses the same check to know the
    // load-time layout is still pending, so it must stay shared.
    if (hasUnpositionedNodes(currentNodes)) {
      autoLayoutTriggeredRef.current = true;
      // Delay slightly to ensure nodes are fully rendered with their measured dimensions
      requestAnimationFrame(() => {
        handleAutoLayout();
      });
    }
  }, [triggerAutoLayoutOnLoad, getNodes, handleAutoLayout]);

  const simulation = useFlowSimulation(nodes, edges);

  // Entering the preview collapses the side panels so the canvas and the
  // preview panel get the full width; they stay closed on exit until reopened.
  const isSimulatingNow = simulation.isSimulating;
  useEffect(() => {
    if (isSimulatingNow) {
      setIsResourcePanelOpen(false);
      setIsOpenResourcePropertiesPanel(false);
    }
  }, [isSimulatingNow, setIsResourcePanelOpen, setIsOpenResourcePropertiesPanel]);

  // Edge under the pointer, tracked so the edge and the node it leads into can
  // be spotlighted (same visual language as the preview's option highlight).
  // Styled via data-id selectors so no node/edge objects change on hover.
  const [hoveredEdge, setHoveredEdge] = useState<{id: string; targetId: string} | null>(null);

  const {isSimulating: isSimulationActive, start: startSimulation, stop: stopSimulation} = simulation;
  const handleToggleSimulation = useCallback((): void => {
    if (isSimulationActive) {
      stopSimulation();
    } else {
      // Drop any hover highlight so it doesn't resurface when the preview ends.
      setHoveredEdge(null);
      startSimulation();
    }
  }, [isSimulationActive, stopSimulation, startSimulation]);

  const handleSave = useCallback((): void => {
    if (isSimulationActive) {
      // Saving during preview: exit the preview first so the persisted viewport
      // is the settled canvas view rather than the preview's zoomed-in camera.
      stopSimulation();
      fitView({padding: 0.2, duration: 0}).then(persistCanvas).catch(persistCanvas);
      return;
    }
    persistCanvas();
  }, [isSimulationActive, stopSimulation, fitView, persistCanvas]);

  // Duplicates the given nodes into the source graph state. Copies come back
  // selected and the originals deselected, so the duplicate is what a follow-up
  // drag, delete, or repeat-duplicate acts on.
  const handleDuplicateNodes = useCallback(
    (nodesToCopy: Node[]): void => {
      const duplicable = nodesToCopy.filter(canDuplicateNode);
      if (duplicable.length === 0) {
        return;
      }
      setNodes((currentNodes: Node[]) => {
        const takenIds = new Set(currentNodes.map((node: Node) => node.id));
        const copies = duplicable.map((node: Node) => {
          const copy = duplicateFlowNode(node, takenIds);
          takenIds.add(copy.id);
          return copy;
        });
        return [...currentNodes.map((node: Node) => (node.selected ? {...node, selected: false} : node)), ...copies];
      });
    },
    [setNodes],
  );

  const handleDuplicateSelection = useCallback((): void => {
    if (isSimulationActive) {
      return;
    }
    handleDuplicateNodes(getNodes().filter((node: Node) => node.selected));
  }, [isSimulationActive, getNodes, handleDuplicateNodes]);

  // Canvas keyboard accelerators: Ctrl/Cmd+S saves, Ctrl/Cmd+D duplicates the
  // selection. Delete/Backspace removal is handled by React Flow itself. Typing
  // contexts are skipped so text editing keeps its native shortcuts.
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent): void => {
      if (!(event.ctrlKey || event.metaKey)) {
        return;
      }
      const target = event.target;
      if (
        target instanceof HTMLElement &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.isContentEditable ||
          target.closest('[contenteditable="true"]'))
      ) {
        return;
      }
      const key = event.key.toLowerCase();
      if (key === 's') {
        // Always claimed, so the browser's save dialog never appears over the
        // builder. An invalid flow still routes through the save handler, which
        // explains the rejection instead of silently dropping the keystroke.
        event.preventDefault();
        if (onSave) {
          handleSave();
        }
      } else if (key === 'd') {
        event.preventDefault();
        handleDuplicateSelection();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleSave, handleDuplicateSelection, onSave]);

  // ── Canvas context menu ──
  const [contextMenuTarget, setContextMenuTarget] = useState<CanvasContextMenuTarget | null>(null);

  const handleNodeContextMenu = useCallback(
    (event: ReactMouseEvent, node: Node): void => {
      event.preventDefault();
      if (isSimulationActive) {
        return;
      }
      setContextMenuTarget({kind: 'node', nodeId: node.id, position: {left: event.clientX, top: event.clientY}});
    },
    [isSimulationActive],
  );

  const handlePaneContextMenu = useCallback(
    (event: ReactMouseEvent | MouseEvent): void => {
      event.preventDefault();
      if (isSimulationActive) {
        return;
      }
      setContextMenuTarget({kind: 'pane', position: {left: event.clientX, top: event.clientY}});
    },
    [isSimulationActive],
  );

  const closeContextMenu = useCallback((): void => setContextMenuTarget(null), []);

  const contextMenuNode: Node | null = useMemo(
    () =>
      contextMenuTarget?.kind === 'node'
        ? (nodes.find((node: Node) => node.id === contextMenuTarget.nodeId) ?? null)
        : null,
    [contextMenuTarget, nodes],
  );

  const contextMenuHasProperties = useMemo(
    () => (contextMenuNode ? buildStepPropertiesResource(contextMenuNode, resources.steps ?? []) !== null : false),
    [contextMenuNode, resources.steps],
  );

  const handleContextDuplicate = useCallback((): void => {
    if (contextMenuNode) {
      handleDuplicateNodes([contextMenuNode]);
    }
  }, [contextMenuNode, handleDuplicateNodes]);

  const handleContextDelete = useCallback((): void => {
    if (contextMenuNode) {
      deleteElements({nodes: [{id: contextMenuNode.id}]}).catch(() => null);
    }
  }, [contextMenuNode, deleteElements]);

  const handleContextOpenProperties = useCallback((): void => {
    if (!contextMenuNode) {
      return;
    }
    const propertiesResource = buildStepPropertiesResource(contextMenuNode, resources.steps ?? []);
    if (!propertiesResource) {
      return;
    }
    setLastInteractedStepId(contextMenuNode.id);
    setLastInteractedResource(propertiesResource);
  }, [contextMenuNode, resources.steps, setLastInteractedStepId, setLastInteractedResource]);

  const handleContextPreviewFromStep = useCallback((): void => {
    if (contextMenuNode) {
      simulation.startAt(contextMenuNode.id);
    }
  }, [contextMenuNode, simulation]);

  // Same list the resource panel's step section shows.
  const addableStepResources = useMemo(
    () => (resources.steps ?? []).filter((step) => step.display?.showOnResourcePanel !== false),
    [resources.steps],
  );

  // Mirrors the resource panel's add-step path (useResourceAdd), but places the
  // step at the right-clicked canvas position instead of the viewport center.
  const handleContextAddStep = useCallback(
    (stepResource: Resource): void => {
      const menuPosition = contextMenuTarget?.position;
      if (!menuPosition) {
        return;
      }
      const position = screenToFlowPosition({x: menuPosition.left, y: menuPosition.top});
      const clonedResource = cloneDeep(stepResource);
      const generatedStep: Step = onStepLoad({
        ...clonedResource,
        data: {components: [], ...(clonedResource.data ?? {})},
        deletable: true,
        id: generateResourceId(clonedResource.type.toLowerCase()),
        position,
      } as Step);
      setNodes((prevNodes: Node[]) => [...prevNodes, generatedStep]);
      onResourceDropOnCanvas(generatedStep, '');
    },
    [contextMenuTarget, screenToFlowPosition, onStepLoad, setNodes, onResourceDropOnCanvas],
  );

  const handleContextAutoLayout = useCallback((): void => {
    handleAutoLayout();
  }, [handleAutoLayout]);

  const handleContextFitView = useCallback((): void => {
    fitView({padding: 0.2, duration: 300}).catch(() => {
      // Ignore fitView errors - fitting is best-effort
    });
  }, [fitView]);

  // Derived presentation state: while simulating, dim everything off the walked
  // path and animate the traversed edges. Returns the original arrays untouched
  // when not simulating so rendering behavior is unchanged. Nodes/edges whose
  // decoration is already correct keep their identity so React Flow's per-node
  // memoization can bail (a hover preview would otherwise re-render every node).
  const displayNodes: Node[] = useMemo(() => {
    if (!simulation.isSimulating) {
      // Self-heals canvas state that picked up simulation styling (e.g. via a
      // drag-collision write while previewing) so nothing stays dimmed.
      return stripSimulationNodeClasses(nodes);
    }
    const pathNodes = new Set(simulation.pathNodeIds);
    const preview = simulation.previewedOption;
    return nodes.map((node: Node) => {
      // While hovering an option, spotlight the node it leads to in the
      // option's kind color so the destination reads at a glance.
      const simulationClasses =
        node.id === preview?.targetNodeId
          ? `simulation-preview-target simulation-kind-${preview.kind}`
          : pathNodes.has(node.id)
            ? 'simulation-path'
            : 'simulation-dimmed';
      const className = withSimulationClasses(node.className, simulationClasses);
      return node.className === className ? node : {...node, className};
    });
  }, [nodes, simulation.isSimulating, simulation.pathNodeIds, simulation.previewedOption]);

  const displayEdges: Edge[] = useMemo(() => {
    if (!simulation.isSimulating) {
      return stripSimulationEdgeClasses(edges);
    }
    const edgeKinds = new Map(simulation.pathEdges.map((traversed) => [traversed.edgeId, traversed.kind]));
    if (simulation.previewedOption) {
      edgeKinds.set(simulation.previewedOption.edgeId, simulation.previewedOption.kind);
    }
    return edges.map((edge: Edge) => {
      const kind = edgeKinds.get(edge.id);
      const animated = Boolean(kind);
      const className = withSimulationClasses(
        edge.className,
        kind ? `simulation-path simulation-kind-${kind}` : 'simulation-dimmed',
      );
      return edge.className === className && edge.animated === animated ? edge : {...edge, animated, className};
    });
  }, [edges, simulation.isSimulating, simulation.pathEdges, simulation.previewedOption]);

  const handleNodeClick = useCallback(
    (event: ReactMouseEvent, node: Node): void => {
      // Bring the clicked node into focus so it is comfortable to configure,
      // especially in large flows viewed zoomed-out. Honors the simulation's
      // static-view toggle — no camera jumps when the user opted out.
      if (simulation.isSimulating && !simulation.followCamera) {
        return;
      }
      // Clicking a stack expands it: the node disappears mid-animation, so
      // focusing it would strand the viewport (the expansion re-layouts).
      if (node.type === EXECUTION_STACK_NODE_TYPE) {
        return;
      }
      // The node header's actions (configure, delete) are inside the node, so
      // their clicks arrive here too. Deleting this way used to throw the
      // canvas to the origin: React Flow queues a fitView and runs it after the
      // next node update, which is the deletion itself, leaving it with no node
      // to fit. Re-centring on a button press is unwanted regardless, so the
      // header actions never move the camera.
      if (event.target instanceof Element && event.target.closest('button')) {
        return;
      }
      // Also skip a node that has already left the canvas, for the same reason.
      if (!getNodes().some((candidate) => candidate.id === node.id)) {
        return;
      }
      fitView({nodes: [{id: node.id}], padding: 0.3, maxZoom: 1.2, duration: 500}).catch(() => {
        // Ignore fitView errors - focusing is best-effort
      });
    },
    [fitView, getNodes, simulation.isSimulating, simulation.followCamera],
  );

  const handleEdgeMouseEnter = useCallback(
    (_event: unknown, edge: Edge): void => {
      // The preview mode has its own path decoration; don't fight it.
      if (simulation.isSimulating) {
        return;
      }
      setHoveredEdge({id: edge.id, targetId: edge.target});
    },
    [simulation.isSimulating],
  );

  const handleEdgeMouseLeave = useCallback((): void => {
    setHoveredEdge(null);
  }, []);

  const handleNodeDragStop = useCallback((): void => {
    const currentNodes = stripSimulationNodeClasses(getNodes());
    const resolvedNodes = resolveCollisions(currentNodes, {
      maxIterations: 10,
      overlapThreshold: 0.5,
      margin: 50,
    });

    // Only update if collision resolution moved any nodes.
    // resolveCollisions returns the original node reference for unmoved nodes,
    // so a reference check is sufficient and avoids iterating positions.
    if (resolvedNodes !== currentNodes && resolvedNodes.some((n, i) => n !== currentNodes[i])) {
      setNodes(resolvedNodes);
    }
  }, [getNodes, setNodes]);

  const handleDragEnd: DragDropEventHandlers['onDragEnd'] = useCallback(
    (event): void => {
      const {source, target} = event.operation;

      if (!source) {
        return;
      }

      const sourceData: DragSourceData = source.data as DragSourceData;
      const targetData = (target?.data ?? {}) as DragTargetData;

      // Check for components that need containers
      const isFormDrop = sourceData.dragged?.type === BlockTypes.Form;
      const isInputDrop = sourceData.dragged?.category === 'FIELD';
      const isWidgetDrop = sourceData.dragged?.resourceType === ResourceTypes.Widget;
      const isCanvasTarget =
        typeof target?.id === 'string' && target.id.startsWith(VisualFlowConstants.FLOW_BUILDER_CANVAS_ID);
      const isViewTarget =
        typeof target?.id === 'string' && target.id.startsWith(VisualFlowConstants.FLOW_BUILDER_VIEW_ID);

      // Form dropped on canvas -> needs View
      if (isFormDrop && isCanvasTarget) {
        pendingDropRef.current = {event, sourceData, targetData};
        setDropScenario('form-on-canvas');
        setIsContainerDialogOpen(true);
        return;
      }

      // Input dropped on canvas -> needs View + Form
      if (isInputDrop && isCanvasTarget) {
        pendingDropRef.current = {event, sourceData, targetData};
        setDropScenario('input-on-canvas');
        setIsContainerDialogOpen(true);
        return;
      }

      // Input dropped on View -> needs Form
      if (isInputDrop && isViewTarget) {
        pendingDropRef.current = {event, sourceData, targetData};
        setDropScenario('input-on-view');
        setIsContainerDialogOpen(true);
        return;
      }

      // Widget dropped on canvas -> needs View
      if (isWidgetDrop && isCanvasTarget) {
        const needsViewContainer = widgetNeedsViewContainer(sourceData.dragged as Widget);

        if (needsViewContainer) {
          pendingDropRef.current = {event, sourceData, targetData};
          setDropScenario('widget-on-canvas');
          setIsContainerDialogOpen(true);
          return;
        }
      }

      // Check if this is a step being added to canvas (not reordering)
      const isStepDrop = sourceData.dragged?.resourceType === ResourceTypes.Step;
      if (isStepDrop && isCanvasTarget && !sourceData.isReordering) {
        // Notify about element addition (for auto-layout hint)
        notifyElementAdded('step');
      }

      // For canceled events or missing target, return early
      if (event.canceled || !target) {
        return;
      }

      // Handle reordering
      if (sourceData.isReordering) {
        if (!sourceData.stepId) {
          return;
        }

        const sourceId = source?.id;

        updateNodeData(sourceData.stepId, (node: Node) => {
          const components: Element[] = (node?.data as StepData)?.components ?? [];

          // Determine which level the dragged element belongs to and only
          // apply move() at that level. Applying move() at both levels can
          // cause the projected source.index to reorder the wrong array.
          const isTopLevel = components.some((c: Element) => c.id === sourceId);

          if (isTopLevel) {
            return {components: move([...components], event)};
          }

          // Element is nested — apply move() only inside the container that holds it
          return {
            components: components.map((component: Element) => {
              if (!component.components) return component;

              const hasElement = component.components.some((c: Element) => c.id === sourceId);
              if (!hasElement) return component;

              return {
                ...component,
                components: move([...component.components], event),
              };
            }),
          };
        });

        requestAnimationFrame(() => {
          updateNodeInternals(sourceData.stepId!);
        });

        return;
      }

      // Handle dropping on canvas
      if (typeof target?.id === 'string' && target.id.startsWith(VisualFlowConstants.FLOW_BUILDER_CANVAS_ID)) {
        addCanvasNode(event, sourceData, targetData);
        return;
      }

      // Handle dropping on View
      if (typeof target?.id === 'string' && target.id.startsWith(VisualFlowConstants.FLOW_BUILDER_VIEW_ID)) {
        addToView(event, sourceData, targetData);
        return;
      }

      // Handle dropping on Form
      if (typeof target?.id === 'string' && target.id.startsWith(VisualFlowConstants.FLOW_BUILDER_FORM_ID)) {
        addToForm(event, sourceData, targetData);
        return;
      }

      // Handle dropping on Stack
      if (typeof target?.id === 'string' && target.id.startsWith(VisualFlowConstants.FLOW_BUILDER_STACK_ID)) {
        addToForm(event, sourceData, targetData);
        return;
      }

      // Handle dropping on an existing element (at specific position)
      if (targetData.isReordering && targetData.stepId && typeof target?.id === 'string') {
        // Check if this is a gap drop zone (between elements)
        const insertBeforeId = (targetData as {insertBeforeElementId?: string}).insertBeforeElementId;
        if (insertBeforeId) {
          addToViewAtIndex(sourceData, targetData.stepId, insertBeforeId);
          return;
        }

        // Dropping on an existing sortable element - insert at that position
        const targetElementId = target.id;

        // Check if the target element is inside a form or stack, at any nesting depth
        const targetNode = getNodes().find((n) => n.id === targetData.stepId);
        const nodeData = targetNode?.data as StepData | undefined;
        const parentContainer = findContainingComponent(nodeData?.components ?? [], targetElementId);

        if (parentContainer) {
          // Target element is inside a form or stack, insert at that position within it
          addToFormAtIndex(sourceData, targetData.stepId, parentContainer.id, targetElementId);
        } else {
          // Phase 1.5: Target is a top-level element in the view, add to view at index
          addToViewAtIndex(sourceData, targetData.stepId, targetElementId);
        }
      }
    },
    [
      updateNodeData,
      updateNodeInternals,
      addCanvasNode,
      addToView,
      addToForm,
      addToViewAtIndex,
      addToFormAtIndex,
      getNodes,
      notifyElementAdded,
    ],
  );

  const handleDragOver: DragDropEventHandlers['onDragOver'] = useCallback(
    (event) => {
      const {source, target} = event.operation;

      if (!source || !target) {
        return;
      }

      if (!source.data.isReordering) {
        return;
      }

      const {data: sourceData} = source;
      const stepId = (sourceData as DragSourceData)?.stepId;

      if (!stepId) {
        return;
      }

      updateNodeData(stepId, (node: Node) => {
        const nodeData = node?.data as StepData | undefined;
        const unorderedComponents: Element[] = nodeData?.components ?? [];

        const reorderedNested = unorderedComponents.map((component: Element) => {
          if (component?.components) {
            return {
              ...component,
              components: move([...component.components], event),
            };
          }

          return component;
        });

        return {
          components: move(reorderedNested, event),
        };
      });

      requestAnimationFrame(() => {
        updateNodeInternals(stepId);
      });
    },
    [updateNodeData, updateNodeInternals],
  );

  const [isDiscardDialogOpen, setIsDiscardDialogOpen] = useState<boolean>(false);

  const handleBackToFlows = useCallback((): void => {
    if (isDirty) {
      setIsDiscardDialogOpen(true);
      return;
    }
    // eslint-disable-next-line @typescript-eslint/no-floating-promises
    navigate(flowRoutes.flows.list());
  }, [flowRoutes, isDirty, navigate]);

  const handleConfirmDiscard = useCallback((): void => {
    setIsDiscardDialogOpen(false);
    // eslint-disable-next-line @typescript-eslint/no-floating-promises
    navigate(flowRoutes.flows.list());
  }, [flowRoutes, navigate]);

  const simulationNode = useMemo(
    () => nodes.find((node: Node) => node.id === simulation.currentNodeId) ?? null,
    [nodes, simulation.currentNodeId],
  );

  // Memoized separately from the simulation panel so simulation state changes
  // (each step, each option hover) don't reconcile the property and validation
  // panel subtrees, and vice versa.
  const editPanels = useMemo(
    () => (
      <>
        <ResourcePropertyPanel
          open={isResourcePropertiesPanelOpen && !openValidationPanel}
          onComponentDelete={deleteComponent}
        />
        <ValidationPanel open={openValidationPanel ?? false} />
      </>
    ),
    [isResourcePropertiesPanelOpen, openValidationPanel, deleteComponent],
  );

  // Memoized so the element reference stays stable across node drag ticks. The
  // simulation panel mounts only while simulating so its data fetching (the
  // application list and design resolution) doesn't run for plain editing.
  const rightPanel = useMemo(
    () => (
      <Box marginLeft={1} display="flex" flexDirection="row">
        {editPanels}
        {simulation.isSimulating && <SimulationStepPreview node={simulationNode} simulation={simulation} />}
      </Box>
    ),
    [editPanels, simulationNode, simulation],
  );

  return (
    <Box
      sx={(theme: Theme) => ({
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        '& .react-flow__edges': {zIndex: 9999},
        '& .react-flow__node': {zIndex: '0 !important'},
        // While a stack's fan preview is out, lift the hovered stack above
        // neighboring nodes and edges so the pushed-out chips render on top of
        // them. The edge layer sits at z-index 9999 above and `.react-flow__nodes`
        // creates no stacking context, so the node competes with it directly.
        '& .react-flow__node:has([data-execution-stack-content]:hover)': {zIndex: '10000 !important'},
        '& .react-flow__handle': {
          width: 10,
          height: 10,
          zIndex: 10000,
          '&:hover': {borderColor: 'var(--oxygen-palette-primary-main)'},
        },
        '& .react-flow__node.simulation-dimmed': {opacity: 0.25, transition: 'opacity 0.3s ease'},
        '& .react-flow__edge.simulation-dimmed': {opacity: 0.15, transition: 'opacity 0.3s ease'},
        '& .react-flow__edge.simulation-kind-action .react-flow__edge-path': {
          stroke: `${theme.palette.primary.main} !important`,
        },
        '& .react-flow__edge.simulation-kind-success .react-flow__edge-path': {
          stroke: `${theme.palette.success.main} !important`,
        },
        '& .react-flow__edge.simulation-kind-incomplete .react-flow__edge-path': {
          stroke: `${theme.palette.warning.main} !important`,
        },
        '& .react-flow__edge.simulation-kind-failure .react-flow__edge-path': {
          stroke: `${theme.palette.error.main} !important`,
        },
        // Mirrors the validation error-pulse (ValidationErrorBoundary.tsx) in the
        // previewed option's kind color — same palette values as the edge strokes above.
        '& .react-flow__node.simulation-preview-target': {
          opacity: 1,
          transition: 'opacity 0.3s ease',
          '&.simulation-kind-action': {'--simulation-preview-color': theme.palette.primary.main},
          '&.simulation-kind-success': {'--simulation-preview-color': theme.palette.success.main},
          '&.simulation-kind-incomplete': {'--simulation-preview-color': theme.palette.warning.main},
          '&.simulation-kind-failure': {'--simulation-preview-color': theme.palette.error.main},
        },
        // The ring sits on the node's own card element so it follows each node type's
        // border radius. Cards are marked with `data-flow-node-surface` because node
        // roots are wrapped in unstyled divs (e.g. ValidationErrorBoundary) whose
        // radius doesn't match.
        '& .react-flow__node.simulation-preview-target [data-flow-node-surface]': {
          outline: '2px solid var(--simulation-preview-color)',
          outlineOffset: '4px',
          animation: 'simulation-preview-target-pulse 1s infinite',
        },
        '@keyframes simulation-preview-target-pulse': {
          '0%': {boxShadow: '0 0 0 0 var(--simulation-preview-color)'},
          '70%': {boxShadow: '0 0 0 15px transparent'},
          '100%': {boxShadow: '0 0 0 0 transparent'},
        },
        // Each edge renders in its own svg layer; lift the hovered one so its
        // highlight stays visible where edges overlap or run close together.
        '& .react-flow__edges svg:has(.react-flow__edge:hover)': {zIndex: '1000 !important'},
        // Spotlight the hovered edge and the node it leads into, mirroring the
        // preview's option highlight. Targeted via data-id so hovering never
        // mutates node/edge objects (React Flow's memoization stays intact).
        // Suppressed while previewing — the simulation owns path decoration.
        ...(hoveredEdge && !simulation.isSimulating
          ? {
              [`& .react-flow__edge[data-id="${hoveredEdge.id}"] .react-flow__edge-path`]: {
                stroke: `${theme.palette.primary.main} !important`,
              },
              [`& .react-flow__node[data-id="${hoveredEdge.targetId}"] [data-flow-node-surface]`]: {
                outline: `2px solid ${theme.palette.primary.main}`,
                outlineOffset: '4px',
                animation: 'edge-hover-target-pulse 1s infinite',
              },
              '@keyframes edge-hover-target-pulse': {
                '0%': {boxShadow: `0 0 0 0 ${theme.palette.primary.main}`},
                '70%': {boxShadow: '0 0 0 15px transparent'},
                '100%': {boxShadow: '0 0 0 0 transparent'},
              },
            }
          : {}),
      })}
    >
      {/* ── Top bar: back button | toolbar | save button ── */}
      <Box sx={{display: 'flex', alignItems: 'center', px: 2, py: 1, flexShrink: 0}}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={14} />}
          onClick={handleBackToFlows}
          sx={{textTransform: 'none', fontSize: '0.8rem', color: 'text.secondary', whiteSpace: 'nowrap'}}
        >
          {t('flows:core.headerPanel.goBack')}
        </Button>

        {/* Centered toolbar */}
        <Box sx={{flex: 1, display: 'flex', justifyContent: 'center'}}>
          <CanvasToolbar
            onAutoLayout={handleAutoLayout}
            onUndo={onUndo}
            onRedo={onRedo}
            canUndo={canUndo}
            canRedo={canRedo}
          />
        </Box>

        <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
          <ValidationBadge errorCount={errorCount} warningCount={warningCount} />
          <Button
            variant="outlined"
            startIcon={simulation.isSimulating ? <Square size={16} /> : <Play size={16} />}
            onClick={handleToggleSimulation}
            data-testid="simulate-flow-button"
          >
            {simulation.isSimulating
              ? t('flows:core.headerPanel.stopSimulation', 'Stop preview')
              : t('flows:core.headerPanel.simulate', 'Preview')}
          </Button>
          <Tooltip
            title={
              hasErrors
                ? t('flows:core.headerPanel.saveDisabledTooltip', 'Fix validation errors before saving')
                : isDirty && onSave
                  ? t('flows:core.headerPanel.unsavedChanges', 'You have unsaved changes')
                  : ''
            }
          >
            <span>
              <Badge
                color="primary"
                variant="dot"
                invisible={!isDirty || hasErrors || !onSave}
                overlap="rectangular"
                anchorOrigin={{vertical: 'top', horizontal: 'right'}}
                data-testid="save-dirty-indicator"
                sx={(theme) => ({
                  '& .MuiBadge-badge': {
                    minWidth: 12,
                    height: 12,
                    borderRadius: '50%',
                    // A ring in the header background separates the blue dot from
                    // the filled Save button so it stays visible.
                    border: `2px solid ${theme.palette.background.default}`,
                    boxShadow: `0 0 0 1px ${theme.palette.primary.main}`,
                  },
                  // The pulse animates `transform`, and an animation outranks the
                  // `scale(0)` MUI hides a badge with — running it unconditionally
                  // would keep the dot on screen for a clean flow. It is scoped to
                  // the visible state, and its keyframes carry MUI's own translate
                  // so the dot stays anchored to the button's corner.
                  '& .MuiBadge-badge:not(.MuiBadge-invisible)': {
                    animation: 'save-dirty-pulse 1.8s ease-in-out infinite',
                  },
                  '@keyframes save-dirty-pulse': {
                    '0%, 100%': {transform: 'scale(1) translate(50%, -50%)', opacity: 1},
                    '50%': {transform: 'scale(1.25) translate(50%, -50%)', opacity: 0.75},
                  },
                })}
              >
                <Button
                  variant="contained"
                  disabled={hasErrors || !onSave}
                  startIcon={<Save size={18} />}
                  // Saving during preview stops it first (handleSave), so the
                  // button stays enabled instead of blocking on the preview.
                  onClick={handleSave}
                  data-testid="save-flow-button"
                >
                  {t('flows:core.headerPanel.save', 'Save')}
                </Button>
              </Badge>
            </span>
          </Tooltip>
        </Box>
      </Box>

      {/* ── Three-column builder area ── */}
      <Box sx={{position: 'relative', flex: 1, overflow: 'hidden', p: 1, pt: 0}}>
        {/* startAt is referentially stable, so providing it does not re-render nodes on simulation state changes */}
        <StepPreviewContext.Provider value={simulation.startAt}>
          <DragDropProvider onDragEnd={handleDragEnd} onDragOver={handleDragOver}>
            <ResourcePanel
              resources={resources}
              open={isResourcePanelOpen}
              onAdd={handleOnAdd}
              disabled={isFlowMetadataLoading}
              flowTitle={flowTitle}
              flowHandle={flowHandle}
              onFlowTitleChange={onFlowTitleChange}
              rightPanel={rightPanel}
              footer={resourcePanelFooter}
            >
              <Droppable
                id={generateResourceId(VisualFlowConstants.FLOW_BUILDER_CANVAS_ID)}
                type={VisualFlowConstants.FLOW_BUILDER_DROPPABLE_CANVAS_ID}
                accept={[...VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES]}
                hideDropZones
                collisionPriority={CollisionPriority.Low}
              >
                <EdgePathsProvider>
                  <VisualFlow
                    nodes={displayNodes}
                    onNodesChange={onNodesChange}
                    edges={displayEdges}
                    edgeTypes={edgeTypes}
                    onEdgesChange={onEdgesChange}
                    onConnect={handleConnect}
                    onNodesDelete={handleNodesDelete}
                    onEdgesDelete={handleEdgesDelete}
                    onNodeDragStop={handleNodeDragStop}
                    onNodeClick={handleNodeClick}
                    onEdgeMouseEnter={handleEdgeMouseEnter}
                    onEdgeMouseLeave={handleEdgeMouseLeave}
                    onNodeContextMenu={handleNodeContextMenu}
                    onPaneContextMenu={handlePaneContextMenu}
                    showMiniMap={isMiniMapVisible}
                    snapToGrid={isSnapToGridEnabled}
                    {...rest}
                  />
                </EdgePathsProvider>
              </Droppable>
            </ResourcePanel>
            <DragOverlay>
              {(source) => {
                const data = source?.data as DragSourceData | undefined;

                if (!data?.isReordering || !data.resource) return null;

                const label = (data.resource as Resource)?.display?.label ?? (data.resource as Resource)?.type;

                return (
                  <Card
                    elevation={3}
                    sx={{
                      px: 2,
                      py: 1.5,
                      minWidth: 120,
                      maxWidth: 280,
                      cursor: 'grabbing',
                      bgcolor: 'background.paper',
                    }}
                  >
                    <CardContent sx={{p: 0, '&:last-child': {pb: 0}}}>
                      <Typography variant="body2" fontWeight={500} noWrap>
                        {label}
                      </Typography>
                    </CardContent>
                  </Card>
                );
              }}
            </DragOverlay>
          </DragDropProvider>
        </StepPreviewContext.Provider>
      </Box>

      <CanvasContextMenu
        target={contextMenuTarget}
        onClose={closeContextMenu}
        canDuplicate={contextMenuNode ? canDuplicateNode(contextMenuNode) : false}
        canDelete={Boolean(contextMenuNode) && contextMenuNode?.deletable !== false}
        hasProperties={contextMenuHasProperties}
        onDuplicate={handleContextDuplicate}
        onDelete={handleContextDelete}
        onOpenProperties={handleContextOpenProperties}
        onPreviewFromStep={handleContextPreviewFromStep}
        addableSteps={addableStepResources}
        onAddStep={handleContextAddStep}
        onAutoLayout={handleContextAutoLayout}
        onFitView={handleContextFitView}
      />
      <FormRequiresViewDialog
        open={isContainerDialogOpen}
        scenario={dropScenario}
        onClose={handleContainerDialogClose}
        onConfirm={handleContainerDialogConfirm}
      />
      <DiscardChangesDialog
        open={isDiscardDialogOpen}
        onClose={() => setIsDiscardDialogOpen(false)}
        onConfirm={handleConfirmDiscard}
      />
    </Box>
  );
}

export default DecoratedVisualFlow;
