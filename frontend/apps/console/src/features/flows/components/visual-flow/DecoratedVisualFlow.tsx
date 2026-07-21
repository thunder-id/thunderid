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

import {CollisionPriority} from '@dnd-kit/abstract';
import {move} from '@dnd-kit/helpers';
import {DragDropProvider, DragOverlay, type DragDropEventHandlers} from '@dnd-kit/react';
import {useIdentityProviders, useSMSProviders} from '@thunderid/configure-connections';
import {Box, Button, Card, CardContent, Tooltip, Typography, type Theme} from '@wso2/oxygen-ui';
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
import {
  type Dispatch,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactElement,
  type SetStateAction,
} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import CanvasToolbar from './CanvasToolbar';
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
import {type Step, type StepData} from '../../models/steps';
import {type Template} from '../../models/templates';
import type {Widget} from '../../models/widget';
import applyAutoLayout from '../../utils/applyAutoLayout';
import computeExecutorConnections from '../../utils/computeExecutorConnections';
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
  ...rest
}: DecoratedVisualFlowPropsInterface): ReactElement {
  useDeleteExecutionResource();
  useConfirmPasswordField();
  useStaticContentField();

  const {toObject, getNodes, getEdges, updateNodeData, fitView} = useReactFlow();
  const updateNodeInternals: UpdateNodeInternals = useUpdateNodeInternals();
  const {deleteComponent} = useComponentDelete();
  const {isResourcePanelOpen, isResourcePropertiesPanelOpen, setIsResourcePanelOpen, setIsOpenResourcePropertiesPanel} =
    useUIPanelState();
  const {notifyElementAdded, onAutoLayout} = useFlowEvents();
  const {isFlowMetadataLoading, metadata, setFlowNodes} = useFlowConfig();
  const {onResourceDropOnCanvas} = useInteractionState();

  // Sync controlled nodes to the shared FlowConfig context so that
  // ValidationProvider (which sits above this ReactFlowProvider) can
  // compute validation notifications from the current node data.
  // Only sync when node data actually changes — skip position-only
  // updates (drag) to avoid unnecessary validation recomputation.
  // Track data references instead of JSON.stringify to avoid O(n) serialization per render.
  const prevNodeDataRefsRef = useRef<Map<string, unknown>>(new Map());

  useEffect(() => {
    let dataChanged = nodes.length !== prevNodeDataRefsRef.current.size;

    if (!dataChanged) {
      for (const node of nodes) {
        if (prevNodeDataRefsRef.current.get(node.id) !== node.data) {
          dataChanged = true;
          break;
        }
      }
    }

    if (dataChanged) {
      const newRefs = new Map<string, unknown>();
      for (const node of nodes) {
        newRefs.set(node.id, node.data);
      }
      prevNodeDataRefsRef.current = newRefs;
      setFlowNodes(nodes);
    }
  }, [nodes, setFlowNodes]);
  const {generateStepElement} = useGenerateStepElement();
  const {t} = useTranslation();
  const navigate = useNavigate();
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
  const handleSave = useCallback((): void => {
    const {viewport} = toObject();
    const canvasData = {
      nodes: stripSimulationNodeClasses(getNodes()),
      edges: stripSimulationEdgeClasses(getEdges()),
      viewport,
    };
    onSave?.(canvasData);
  }, [toObject, getNodes, getEdges, onSave]);

  const handleAutoLayout = useCallback((): void => {
    const currentNodes = stripSimulationNodeClasses(getNodes());
    const currentEdges = getEdges();
    applyAutoLayout(currentNodes, currentEdges, {
      nodeSpacing: 100,
      rankSpacing: 160,
      offsetX: 50,
      offsetY: 50,
    })
      .then((layoutedNodes) => {
        setNodes(layoutedNodes);
        requestAnimationFrame(() => {
          fitView({padding: 0.2, duration: 300}).catch(() => {
            // Ignore fitView errors - layout is still applied
          });
        });
      })
      .catch(() => {
        // Layout failed, keep original positions
      });
  }, [getNodes, getEdges, setNodes, fitView]);

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

    // Check if nodes need auto-layout by detecting if multiple nodes are at the same position
    // (which happens when layout data is missing and all default to {x: 0, y: 0})
    const nodesAtOrigin = currentNodes.filter((node) => node.position.x === 0 && node.position.y === 0);

    // If more than one node is at the origin, we need auto-layout
    const needsAutoLayout = nodesAtOrigin.length > 1;

    if (needsAutoLayout) {
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

  const {isSimulating: isSimulationActive, start: startSimulation, stop: stopSimulation} = simulation;
  const handleToggleSimulation = useCallback((): void => {
    if (isSimulationActive) {
      stopSimulation();
    } else {
      startSimulation();
    }
  }, [isSimulationActive, stopSimulation, startSimulation]);

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
    (_event: unknown, node: Node): void => {
      // Bring the clicked node into focus so it is comfortable to configure,
      // especially in large flows viewed zoomed-out. Honors the simulation's
      // static-view toggle — no camera jumps when the user opted out.
      if (simulation.isSimulating && !simulation.followCamera) {
        return;
      }
      fitView({nodes: [{id: node.id}], padding: 0.3, maxZoom: 1.2, duration: 500}).catch(() => {
        // Ignore fitView errors - focusing is best-effort
      });
    },
    [fitView, simulation.isSimulating, simulation.followCamera],
  );

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

  const handleBackToFlows = useCallback((): void => {
    // eslint-disable-next-line @typescript-eslint/no-floating-promises
    navigate('/flows');
  }, [navigate]);

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
        // Mirrors the validation error-pulse (ValidationErrorBoundary.scss) in the
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
        // border radius. Cards are addressed by class because node roots are wrapped
        // in unstyled divs (e.g. ValidationErrorBoundary) whose radius doesn't match.
        ['& .react-flow__node.simulation-preview-target .flow-builder-step, ' +
        '& .react-flow__node.simulation-preview-target .execution-minimal-step, ' +
        '& .react-flow__node.simulation-preview-target .flow-builder-rule, ' +
        '& .react-flow__node.simulation-preview-target .MuiFab-root']: {
          outline: '2px solid var(--simulation-preview-color)',
          outlineOffset: '4px',
          animation: 'simulation-preview-target-pulse 1s infinite',
        },
        '@keyframes simulation-preview-target-pulse': {
          '0%': {boxShadow: '0 0 0 0 var(--simulation-preview-color)'},
          '70%': {boxShadow: '0 0 0 15px transparent'},
          '100%': {boxShadow: '0 0 0 0 transparent'},
        },
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
          <CanvasToolbar onAutoLayout={handleAutoLayout} />
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
                : isSimulationActive
                  ? t('flows:core.headerPanel.saveDisabledDuringPreview', 'Stop the preview before saving')
                  : ''
            }
          >
            <span>
              <Button
                variant="contained"
                // Saving mid-preview would persist the preview's zoomed-in camera
                // as the flow's viewport — stop the preview first.
                disabled={hasErrors || !onSave || isSimulationActive}
                startIcon={<Save size={18} />}
                onClick={handleSave}
                data-testid="save-flow-button"
              >
                {t('flows:core.headerPanel.save')}
              </Button>
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
            >
              <Droppable
                id={generateResourceId(VisualFlowConstants.FLOW_BUILDER_CANVAS_ID)}
                type={VisualFlowConstants.FLOW_BUILDER_DROPPABLE_CANVAS_ID}
                accept={[...VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES]}
                hideDropZones
                collisionPriority={CollisionPriority.Low}
              >
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
                  {...rest}
                />
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

      <FormRequiresViewDialog
        open={isContainerDialogOpen}
        scenario={dropScenario}
        onClose={handleContainerDialogClose}
        onConfirm={handleContainerDialogConfirm}
      />
    </Box>
  );
}

export default DecoratedVisualFlow;
