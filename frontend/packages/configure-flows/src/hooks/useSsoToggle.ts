// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import type {Dispatch, MouseEvent as ReactMouseEvent, SetStateAction} from 'react';
import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import useFlowConfig from './useFlowConfig';
import useInteractionState from './useInteractionState';
import useUIPanelState from './useUIPanelState';
import type {Resource, Resources} from '../models/resources';
import {ResourceTypes} from '../models/resources';
import type {Step, StepData} from '../models/steps';
import {ExecutionTypes, StepCategories} from '../models/steps';
import {resolveCollisions} from '../utils/resolveCollisions';
import {
  deriveSsoState,
  disableSso,
  enableSso,
  findJoinCandidates,
  isSessionNode,
  isSsoCheckNode,
} from '../utils/ssoGraphTransforms';
import type {JoinResolution, SsoState} from '../utils/ssoGraphTransforms';

export interface SsoPlacementState {
  active: boolean;
  candidateEdgeIds: string[];
}

export interface SsoFocusRequest {
  ssoCheckId: string;
  sessionId: string;
  /** Resource descriptor of the inserted SSO check, for selection and opening its properties panel. */
  resource: Resource;
}

export interface UseSsoToggleProps {
  nodes: Node[];
  edges: Edge[];
  setNodes: Dispatch<SetStateAction<Node[]>>;
  setEdges: Dispatch<SetStateAction<Edge[]>>;
  resources: Resources;
  showInfo: (message: string) => void;
  showSuccess: (message: string) => void;
}

export interface UseSsoToggleReturn {
  ssoState: SsoState;
  joinResolution: JoinResolution;
  placement: SsoPlacementState;
  isConfirmDialogOpen: boolean;
  focusRequest: SsoFocusRequest | null;
  clearFocusRequest: () => void;
  handleEnable: () => void;
  handleDisableRequest: () => void;
  handleConfirmDisable: () => void;
  handleCloseConfirmDialog: () => void;
  handleCancelPlacement: () => void;
  handleEdgeClick: (event: ReactMouseEvent, edge: Edge) => void;
}

const INACTIVE_PLACEMENT: SsoPlacementState = {active: false, candidateEdgeIds: []};

function findExecutorResource(resources: Resources, executorName: string): Step | undefined {
  return resources.executors?.find(
    (executor) => (executor.data as StepData | undefined)?.action?.executor?.name === executorName,
  );
}

/**
 * Builds the resource descriptor for a freshly inserted execution node so it
 * can be selected and its properties panel opened (same shape the Execution
 * node component passes on click).
 */
function toExecutionResource(node: Node): Resource {
  const data = node.data as StepData & {display?: {label?: string; image?: string}};
  return {
    category: StepCategories.Workflow,
    data,
    display: {
      image: data.display?.image ?? '',
      label: data.display?.label ?? '',
    },
    id: node.id,
    resourceType: ResourceTypes.Step,
    type: 'EXECUTION',
  } as unknown as Resource;
}

/**
 * Orchestrates the "Enable SSO" toggle for login flows: derives the toggle
 * state from the graph, runs the enable/disable transformations, and manages
 * the interactive placement mode used when the join point is ambiguous.
 */
const useSsoToggle = ({
  nodes,
  edges,
  setNodes,
  setEdges,
  resources,
  showInfo,
  showSuccess,
}: UseSsoToggleProps): UseSsoToggleReturn => {
  const {t} = useTranslation();
  const {edgeStyle} = useFlowConfig();
  const {lastInteractedResource, lastInteractedStepId} = useInteractionState();
  const {setIsOpenResourcePropertiesPanel} = useUIPanelState();

  const [placement, setPlacement] = useState<SsoPlacementState>(INACTIVE_PLACEMENT);
  const [isConfirmDialogOpen, setIsConfirmDialogOpen] = useState<boolean>(false);
  const [focusRequest, setFocusRequest] = useState<SsoFocusRequest | null>(null);

  // The graph is read through a ref inside the handlers so their identity
  // stays stable across node-drag ticks.
  const graphRef = useRef<{nodes: Node[]; edges: Edge[]}>({edges, nodes});
  useEffect(() => {
    graphRef.current = {edges, nodes};
  }, [nodes, edges]);

  // The derivation only depends on the graph structure, never on node
  // positions, so it is keyed on a cheap structural signature: drag ticks
  // (position-only changes) cost one string build here instead of the full
  // derive-and-serialize, and the derived object identity stays stable so
  // the memoized toggle UI doesn't re-render for equivalent graphs.
  const structuralSignature = useMemo(() => {
    let signature = '';
    for (const node of nodes) {
      signature += `${node.id}|${node.type ?? ''}|${(node.data as StepData | undefined)?.action?.executor?.name ?? ''};`;
    }
    for (const edge of edges) {
      signature += `${edge.id}|${edge.source}|${edge.sourceHandle ?? ''}|${edge.target};`;
    }
    return signature;
  }, [nodes, edges]);

  const {ssoState, joinResolution} = useMemo(
    () => ({joinResolution: findJoinCandidates(nodes, edges), ssoState: deriveSsoState(nodes)}),
    // The signature captures every structural input of the derivation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [structuralSignature],
  );

  const ssoResources = useMemo(() => {
    const ssoCheck = findExecutorResource(resources, ExecutionTypes.SSOCheck);
    const session = findExecutorResource(resources, ExecutionTypes.Session);
    return ssoCheck && session ? {session, ssoCheck} : null;
  }, [resources]);

  const applyEnable = useCallback(
    (joinNodeId?: string): void => {
      if (!ssoResources) {
        return;
      }

      const {nodes: currentNodes, edges: currentEdges} = graphRef.current;
      const result = enableSso(currentNodes, currentEdges, ssoResources, edgeStyle, joinNodeId);
      if (!result.newSsoCheckId || !result.newSessionId) {
        return;
      }

      const resolvedNodes = resolveCollisions(result.nodes, {
        margin: 50,
        maxIterations: 10,
        overlapThreshold: 0.5,
      });
      setNodes(resolvedNodes);
      setEdges(result.edges);
      setPlacement(INACTIVE_PLACEMENT);

      const insertedSsoCheck = result.nodes.find((node) => node.id === result.newSsoCheckId);
      if (insertedSsoCheck) {
        setFocusRequest({
          resource: toExecutionResource(insertedSsoCheck),
          sessionId: result.newSessionId,
          ssoCheckId: result.newSsoCheckId,
        });
      }

      showInfo(
        t(
          'flows:sso.enabledSnackbar',
          'SSO enabled. A session check now runs after Start, and sessions are saved before the flow completes.',
        ),
      );
    },
    [ssoResources, edgeStyle, setNodes, setEdges, showInfo, t],
  );

  const handleEnable = useCallback((): void => {
    if (ssoState.enabled) {
      return;
    }

    if (joinResolution.status === 'ok') {
      applyEnable();
      return;
    }

    if (joinResolution.status === 'ambiguous') {
      // Don't guess the join point; let the user click one of the candidate edges.
      setPlacement({active: true, candidateEdgeIds: joinResolution.candidateEdgeIds});
    }
  }, [ssoState.enabled, joinResolution, applyEnable]);

  const handleDisableRequest = useCallback((): void => {
    if (ssoState.enabled) {
      setIsConfirmDialogOpen(true);
    }
  }, [ssoState.enabled]);

  const handleConfirmDisable = useCallback((): void => {
    const {nodes: currentNodes, edges: currentEdges} = graphRef.current;
    const result = disableSso(currentNodes, currentEdges);

    // Close the properties panel when it is showing one of the removed nodes,
    // so it doesn't linger with a stale, dangling-reference view.
    const removedIds = new Set(
      currentNodes.filter((node) => isSsoCheckNode(node) || isSessionNode(node)).map((node) => node.id),
    );
    if (removedIds.has(lastInteractedStepId) || removedIds.has(lastInteractedResource?.id ?? '')) {
      setIsOpenResourcePropertiesPanel(false);
    }

    setNodes(result.nodes);
    setEdges(result.edges);
    setIsConfirmDialogOpen(false);
    showSuccess(
      t('flows:sso.disabledSnackbar', {
        count: result.removedCount,
        defaultValue_one: 'SSO disabled. {{count}} checkpoint was removed and the flow reconnected.',
        defaultValue_other: 'SSO disabled. {{count}} checkpoints were removed and the flow reconnected.',
      }),
    );
  }, [
    lastInteractedStepId,
    lastInteractedResource,
    setIsOpenResourcePropertiesPanel,
    setNodes,
    setEdges,
    showSuccess,
    t,
  ]);

  const handleCloseConfirmDialog = useCallback((): void => {
    setIsConfirmDialogOpen(false);
  }, []);

  const handleCancelPlacement = useCallback((): void => {
    setPlacement(INACTIVE_PLACEMENT);
  }, []);

  const handleEdgeClick = useCallback(
    (_event: ReactMouseEvent, edge: Edge): void => {
      if (!placement.active || !placement.candidateEdgeIds.includes(edge.id)) {
        return;
      }
      applyEnable(edge.target);
    },
    [placement, applyEnable],
  );

  const clearFocusRequest = useCallback((): void => {
    setFocusRequest(null);
  }, []);

  // Esc cancels placement mode.
  useEffect(() => {
    if (!placement.active) {
      return undefined;
    }
    const handleKeyDown = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        setPlacement(INACTIVE_PLACEMENT);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [placement.active]);

  return {
    clearFocusRequest,
    focusRequest,
    handleCancelPlacement,
    handleCloseConfirmDialog,
    handleConfirmDisable,
    handleDisableRequest,
    handleEdgeClick,
    handleEnable,
    isConfirmDialogOpen,
    joinResolution,
    placement,
    ssoState,
  };
};

export default useSsoToggle;
