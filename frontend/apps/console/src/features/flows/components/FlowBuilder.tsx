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

import {useIdentityProviders, useSMSProviders} from '@thunderid/configure-connections';
import {Alert, Box, Snackbar, Stack} from '@wso2/oxygen-ui';
import type {Edge, Node, NodeChange} from '@xyflow/react';
import {useEdgesState, useNodesState, useUpdateNodeInternals} from '@xyflow/react';
import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import '@xyflow/react/dist/style.css';

import SsoDisableConfirmDialog from '../components/SsoDisableConfirmDialog';
import SsoToggle from '../components/SsoToggle';
import CompactStacksContext from '../context/CompactStacksContext';
import useEdgeGeneration from '../hooks/useEdgeGeneration';
import useElementAddition from '../hooks/useElementAddition';
import useFlowHistory, {computeGraphSignature} from '../hooks/useFlowHistory';
import useFlowInitialization from '../hooks/useFlowInitialization';
import useFlowNaming from '../hooks/useFlowNaming';
import useFlowSave from '../hooks/useFlowSave';
import useNodeTypes from '../hooks/useNodeTypes';
import useSnackbarNotifications from '../hooks/useSnackbarNotifications';
import useSsoToggle from '../hooks/useSsoToggle';
import useTemplateAndWidgetLoading from '../hooks/useTemplateAndWidgetLoading';
import {hasUnpositionedNodes} from '../utils/applyAutoLayout';
import collapseExecutorChains, {getExecutionStackHeadId} from '../utils/compactGraphTransforms';
import {mutateComponents} from '../utils/componentMutations';
import GradientBorderButton from '@/features/applications/components/GradientBorderButton';
import useGetFlowBuilderResources from '@/features/flows/api/useGetFlowBuilderResources';
import useGetFlowById from '@/features/flows/api/useGetFlowById';
import FlowCanvas from '@/features/flows/components/FlowCanvas';
import FlowConstants from '@/features/flows/constants/FlowConstants';
import useFlowConfig from '@/features/flows/hooks/useFlowConfig';
import useFlowEvents from '@/features/flows/hooks/useFlowEvents';
import useValidationStatus from '@/features/flows/hooks/useValidationStatus';

import {GRAPH_VALIDATION_RULES} from '@/features/flows/validation/validation-rules';


function FlowBuilder() {
  const {flowId} = useParams<{flowId: string}>();
  const [nodes, setNodes, defaultOnNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const {t} = useTranslation();

  const {data: resources} = useGetFlowBuilderResources();
  const {edgeStyle, isVerboseMode, setGraphValidationRules} = useFlowConfig();
  const {triggerAutoLayout, onRestoreFromHistory, onElementAdded} = useFlowEvents();
  const {isValid: isFlowValid, setOpenValidationPanel} = useValidationStatus();
  const updateNodeInternals = useUpdateNodeInternals();

  // View-mode relayouts (the compact toggle and stack expand/collapse) move
  // nodes without the user editing anything, which would otherwise light up the
  // Save indicator. When the flow is clean as one starts, this flag rebases the
  // clean baseline onto whatever the relayout settles into.
  const [isBaselineRebasePending, setIsBaselineRebasePending] = useState<boolean>(false);

  // Fetch the existing flow if flowId is provided (editing an existing flow)
  const {data: existingFlowData, isLoading: isLoadingExistingFlow} = useGetFlowById(flowId);

  // Determine if we're editing an existing flow
  const isEditingExistingFlow = Boolean(flowId && existingFlowData);

  // Flow naming hook
  const {flowName, flowHandle, needsAutoLayout, setNeedsAutoLayout, handleFlowNameChange} = useFlowNaming({
    existingFlowData: existingFlowData as {name?: string; handle?: string} | undefined,
  });

  // Snackbar notifications hook
  const {
    errorSnackbar,
    successSnackbar,
    infoSnackbar,
    showError,
    showSuccess,
    showInfo,
    handleCloseErrorSnackbar,
    handleCloseSuccessSnackbar,
    handleCloseInfoSnackbar,
  } = useSnackbarNotifications();

  const handleAutoLayoutClick = useCallback(() => {
    triggerAutoLayout();
    handleCloseInfoSnackbar();
  }, [triggerAutoLayout, handleCloseInfoSnackbar]);

  // Edge generation hook
  const {generateEdges, validateEdges} = useEdgeGeneration({
    startStepId: FlowConstants.START_STEP_ID,
    endStepId: FlowConstants.END_STEP_ID,
  });

  // Flow initialization hook
  const {generateSteps, getBlankTemplateComponents} = useFlowInitialization({
    resources,
    flowId,
    existingFlowData,
    isLoadingExistingFlow,
    setNodes,
    setEdges,
    updateNodeInternals,
    generateEdges,
    validateEdges,
    edgeStyle,
    onNeedsAutoLayout: setNeedsAutoLayout,
  });

  const {isPending: isIdentityProvidersPending} = useIdentityProviders();
  const {isPending: isSMSProvidersPending} = useSMSProviders();
  // Element addition hook
  const {handleAddElementToView, handleAddElementToForm} = useElementAddition({
    setNodes,
    updateNodeInternals,
  });

  // Node types hook
  const {nodeTypes, edgeTypes} = useNodeTypes({
    steps: resources.steps,
    resources,
    onAddElementToView: handleAddElementToView,
    onAddElementToForm: handleAddElementToForm,
  });

  // Template and widget loading hook
  const {handleStepLoad, handleTemplateLoad, handleWidgetLoad, handleResourceAdd} = useTemplateAndWidgetLoading({
    resources,
    generateSteps,
    generateEdges,
    validateEdges,
    getBlankTemplateComponents,
    setNodes,
    updateNodeInternals,
  });

  const flowType = (existingFlowData as {flowType?: string} | undefined)?.flowType ?? 'AUTHENTICATION';

  // The SSO pairing rules only apply to AUTHENTICATION flows; other flow
  // types run without graph-level rules.
  useEffect(() => {
    setGraphValidationRules(flowType === 'AUTHENTICATION' ? GRAPH_VALIDATION_RULES : []);
  }, [flowType, setGraphValidationRules]);

  // Undo/redo history over the canvas graph.
  const {undo, redo, canUndo, canRedo, settledSignature, resetHistory} = useFlowHistory({
    edges,
    maxHistoryItems: FlowConstants.MAX_HISTORY_ITEMS,
    nodes,
    setEdges,
    setNodes,
  });

  // Unsaved-changes tracking. The flow is considered clean until the user first
  // interacts with the builder: the graph keeps mutating on its own after load
  // (metadata resolution, field normalization, node internals, auto-layout), and
  // none of that should count as an edit. The baseline signature is frozen at the
  // first user interaction — every real edit begins with a pointer/key event, so
  // the pre-edit graph is always captured — and re-frozen after each save.
  // Dirtiness compares against the history hook's settled signature, so no
  // per-render (or per-drag-tick) re-serialization of the graph is needed.
  const [savedSignature, setSavedSignature] = useState<string | null>(null);
  const isDirty = savedSignature !== null && settledSignature !== null && settledSignature !== savedSignature;

  // Apply a pending rebase once the relayout settles (state adjustment during
  // render rather than an effect, so no extra render pass).
  const [settledSignatureAtLastRebaseCheck, setSettledSignatureAtLastRebaseCheck] = useState<string | null>(
    settledSignature,
  );
  if (settledSignature !== settledSignatureAtLastRebaseCheck) {
    setSettledSignatureAtLastRebaseCheck(settledSignature);
    if (isBaselineRebasePending && settledSignature !== null) {
      setIsBaselineRebasePending(false);
      if (savedSignature !== null) {
        setSavedSignature(settledSignature);
      }
    }
  }

  const resetHistoryRef = useRef(resetHistory);
  useEffect(() => {
    resetHistoryRef.current = resetHistory;
  }, [resetHistory]);

  // The graph keeps mutating on its own until the load has fully settled: the
  // flow request resolves, the connection lookups that fill in a sole provider
  // resolve, and the load-time auto-layout positions a flow stored without
  // layout data. Freezing the clean baseline before all of that lands would
  // make those app-driven changes look like edits, so the freeze waits.
  const isLoadSettlingRef = useRef<boolean>(true);
  useEffect(() => {
    isLoadSettlingRef.current =
      isLoadingExistingFlow || isIdentityProvidersPending || isSMSProvidersPending || hasUnpositionedNodes(nodes);
  }, [isLoadingExistingFlow, isIdentityProvidersPending, isSMSProvidersPending, nodes]);

  const hasInteractedRef = useRef<boolean>(false);
  useEffect(() => {
    const freezeBaseline = (): void => {
      // A real interaction cancels any pending view-mode rebase, so an edit
      // made while a relayout is still settling is never absorbed into the
      // clean baseline.
      setIsBaselineRebasePending(false);
      if (hasInteractedRef.current || isLoadSettlingRef.current) {
        // While the load is still settling the baseline is left unfrozen (and
        // the flow stays clean); the next interaction after it settles takes
        // the baseline instead.
        return;
      }
      hasInteractedRef.current = true;
      // Rebase history on the settled pre-edit graph (discarding any entries
      // recorded while it was still settling) and take it as the clean baseline.
      setSavedSignature(resetHistoryRef.current());
    };
    window.addEventListener('pointerdown', freezeBaseline, true);
    window.addEventListener('keydown', freezeBaseline, true);
    return () => {
      window.removeEventListener('pointerdown', freezeBaseline, true);
      window.removeEventListener('keydown', freezeBaseline, true);
    };
  }, []);

  // Rebase the clean baseline on the canvas snapshot that was actually
  // persisted — the user may have kept editing while the save request was in
  // flight, and those edits must stay dirty. History is intentionally NOT
  // reset here: undoing past a save point simply makes the flow dirty again.
  const handleSaved = useCallback((savedCanvas: {nodes: Node[]; edges: Edge[]}) => {
    hasInteractedRef.current = true;
    setSavedSignature(computeGraphSignature(savedCanvas.nodes, savedCanvas.edges));
  }, []);

  // Flow save hook
  const {handleSave} = useFlowSave({
    flowId,
    isEditingExistingFlow,
    isFlowValid,
    flowName,
    flowHandle,
    flowType,
    showError,
    showSuccess,
    setOpenValidationPanel,
    onSaved: handleSaved,
  });


  // SSO toggle orchestration (enable/disable transformations, placement mode)
  const sso = useSsoToggle({
    nodes,
    edges,
    setNodes,
    setEdges,
    resources,
    showInfo,
    showSuccess,
  });

  // In compact mode, runs of consecutive non-branching executors are collapsed
  // into synthetic stack nodes for display only; the underlying graph state is
  // untouched. Measured dimensions of the synthetic nodes are kept in local
  // state (fed by React Flow's dimension changes below): React Flow only
  // renders edges once both endpoint nodes are measured, so dropping the
  // dimension roundtrip would silently hide every edge touching a stack.
  const [stackDimensions, setStackDimensions] = useState<Map<string, {height: number; width: number}>>(
    () => new Map<string, {height: number; width: number}>(),
  );

  // Stacks the user opened into their individual chips. Reset when leaving
  // compact mode so the next visit starts collapsed again (state adjustment
  // during render, per the React "adjusting state when props change" pattern).
  const [expandedStackIds, setExpandedStackIds] = useState<Set<string>>(() => new Set<string>());
  const [prevVerboseForExpansion, setPrevVerboseForExpansion] = useState<boolean>(isVerboseMode);
  if (prevVerboseForExpansion !== isVerboseMode) {
    setPrevVerboseForExpansion(isVerboseMode);
    if (isVerboseMode && expandedStackIds.size > 0) {
      setExpandedStackIds(new Set<string>());
    }
    // Switching view mode relayouts the canvas; that is not a user edit.
    if (!isDirty) {
      setIsBaselineRebasePending(true);
    }
  }

  const handleExpandStack = useCallback(
    (stackId: string, memberIds: string[]) => {
      setExpandedStackIds((current) => new Set(current).add(stackId));
      // Fan the members out from the head position so the freshly split chips
      // do not sit on top of each other while the auto-layout below settles.
      setNodes((currentNodes) => {
        const headNode = currentNodes.find((node) => node.id === memberIds[0]);
        if (!headNode) {
          return currentNodes;
        }
        const spacing = 64;
        const positionById = new Map(
          memberIds.map((memberId, index) => [
            memberId,
            {x: headNode.position.x + index * spacing, y: headNode.position.y},
          ]),
        );
        return currentNodes.map((node) => {
          const position = positionById.get(node.id);
          return position ? {...node, position} : node;
        });
      });
      // Re-run the compact auto-layout once the split chips are committed so
      // the canvas makes room for them. Expanding is a view operation, so the
      // relayout it causes must not mark the flow dirty.
      if (!isDirty) {
        setIsBaselineRebasePending(true);
      }
      requestAnimationFrame(() => triggerAutoLayout());
    },
    [setNodes, triggerAutoLayout, isDirty],
  );

  const handleCollapseStack = useCallback(
    (stackId: string) => {
      setExpandedStackIds((current) => {
        const next = new Set(current);
        next.delete(stackId);
        return next;
      });
      if (!isDirty) {
        setIsBaselineRebasePending(true);
      }
      requestAnimationFrame(() => triggerAutoLayout());
    },
    [triggerAutoLayout, isDirty],
  );

  const compactStacksContextValue = useMemo(
    () => ({
      collapseStack: handleCollapseStack,
      expandStack: handleExpandStack,
      expandedHeadIdToStackId: new Map(
        [...expandedStackIds].map((stackId) => [getExecutionStackHeadId(stackId), stackId]),
      ),
    }),
    [handleCollapseStack, handleExpandStack, expandedStackIds],
  );

  const compactGraph = useMemo(
    () => (isVerboseMode ? null : collapseExecutorChains(nodes, edges, expandedStackIds)),
    [nodes, edges, isVerboseMode, expandedStackIds],
  );
  const displayNodes = useMemo(() => {
    if (!compactGraph) {
      return nodes;
    }
    return compactGraph.nodes.map((node) => {
      const dimensions = stackDimensions.get(node.id);
      return dimensions ? {...node, measured: dimensions} : node;
    });
  }, [compactGraph, nodes, stackDimensions]);
  const baseDisplayEdges = compactGraph?.edges ?? edges;

  const stackMembersRef = useRef<Map<string, string[]>>(new Map<string, string[]>());
  useEffect(() => {
    stackMembersRef.current = compactGraph?.stackMembersById ?? new Map<string, string[]>();
  }, [compactGraph]);

  // Canvas changes that address a synthetic stack node are translated onto its
  // member nodes so drags and deletions land on the real graph state.
  const onNodesChange = useCallback(
    (changes: NodeChange[]) => {
      const stackMembers = stackMembersRef.current;
      if (stackMembers.size === 0) {
        defaultOnNodesChange(changes);
        return;
      }
      const expandedChanges = changes.flatMap((change): NodeChange[] => {
        const changeId = 'id' in change ? change.id : undefined;
        const memberIds = changeId ? stackMembers.get(changeId) : undefined;
        if (!memberIds) {
          return [change];
        }
        if (change.type === 'position' || change.type === 'select' || change.type === 'remove') {
          return memberIds.map((memberId) => ({...change, id: memberId}));
        }
        if (change.type === 'dimensions' && change.dimensions && changeId) {
          // Synthetic nodes have no state counterpart; record their measured
          // size locally so React Flow sees them as initialized and keeps
          // rendering their edges.
          const {dimensions} = change;
          setStackDimensions((current) => {
            const known = current.get(changeId);
            if (known?.width === dimensions.width && known?.height === dimensions.height) {
              return current;
            }
            const next = new Map(current);
            next.set(changeId, {height: dimensions.height, width: dimensions.width});
            return next;
          });
        }
        return [];
      });
      defaultOnNodesChange(expandedChanges);
    },
    [defaultOnNodesChange],
  );

  // Undo/redo keyboard shortcuts. Ignore edits inside text fields / rich text so
  // native text undo keeps working there.
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent): void => {
      if (!(event.ctrlKey || event.metaKey)) {
        return;
      }
      const target = event.target as HTMLElement | null;
      if (
        target &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.isContentEditable ||
          target.closest('[contenteditable="true"]'))
      ) {
        return;
      }
      const key = event.key.toLowerCase();
      if (key === 'z' && !event.shiftKey) {
        event.preventDefault();
        undo();
      } else if ((key === 'z' && event.shiftKey) || key === 'y') {
        event.preventDefault();
        redo();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [undo, redo]);

  // Warn before leaving (tab close / reload) with unsaved changes. Dirtiness is
  // computed at unload time from the live graph, so the check stays exact even
  // when the latest edit is still inside the history debounce window.
  const graphRef = useRef<{nodes: Node[]; edges: Edge[]}>({edges, nodes});
  useEffect(() => {
    graphRef.current = {edges, nodes};
  }, [nodes, edges]);

  const savedSignatureRef = useRef<string | null>(savedSignature);
  useEffect(() => {
    savedSignatureRef.current = savedSignature;
  }, [savedSignature]);

  useEffect(() => {
    const handleBeforeUnload = (event: BeforeUnloadEvent): void => {
      if (savedSignatureRef.current === null) {
        return;
      }
      const {nodes: liveNodes, edges: liveEdges} = graphRef.current;
      if (computeGraphSignature(liveNodes, liveEdges) === savedSignatureRef.current) {
        return;
      }
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, []);

  // Handle restore from history event
  useEffect(
    () =>
      onRestoreFromHistory((restoredNodes, restoredEdges) => {
        setNodes(restoredNodes);
        setEdges(restoredEdges);
      }),
    [onRestoreFromHistory, setNodes, setEdges],
  );

  // Listen for element added events to show auto-layout hint
  useEffect(
    () =>
      onElementAdded((type) => {
        if (type === 'step' || type === 'widget' || type === 'template') {
          showInfo(t('flows:core.canvas.hints.autoLayout'));
        }
      }),
    [onElementAdded, showInfo, t],
  );

  // Update edge types when edge style changes
  useEffect(() => {
    setEdges((currentEdges) =>
      currentEdges.map((edge) => ({
        ...edge,
        type: edgeStyle,
      })),
    );
  }, [edgeStyle, setEdges]);

  // While the SSO placement mode is active, spotlight the candidate join edges
  // and dim the rest (same visual language as the simulation path decoration).
  // Edge ids survive the compact-mode rewiring, so the candidate lookup works
  // on the display edges in both modes.
  const displayEdges = useMemo(() => {
    if (!sso.placement.active) {
      return baseDisplayEdges;
    }
    const candidateEdgeIds = new Set(sso.placement.candidateEdgeIds);
    return baseDisplayEdges.map((edge) => ({
      ...edge,
      className: candidateEdgeIds.has(edge.id) ? 'sso-placement-candidate' : 'sso-placement-dimmed',
    }));
  }, [baseDisplayEdges, sso.placement]);

  const isReadOnlyFlow = Boolean(existingFlowData?.isReadOnly);

  // Memoized so the resource panel (which renders this as its footer and is
  // itself memoized) is not re-rendered on unrelated graph changes like drags.
  const ssoToggleFooter = useMemo(
    () =>
      flowType === 'AUTHENTICATION' ? (
        <SsoToggle
          ssoState={sso.ssoState}
          joinResolution={sso.joinResolution}
          placement={sso.placement}
          isReadOnly={isReadOnlyFlow}
          focusRequest={sso.focusRequest}
          onFocusHandled={sso.clearFocusRequest}
          onEnable={sso.handleEnable}
          onDisableRequest={sso.handleDisableRequest}
        />
      ) : undefined,
    [
      flowType,
      isReadOnlyFlow,
      sso.ssoState,
      sso.joinResolution,
      sso.placement,
      sso.focusRequest,
      sso.clearFocusRequest,
      sso.handleEnable,
      sso.handleDisableRequest,
    ],
  );

  return (
    <Box
      sx={(theme) => ({
        width: '100%',
        ['& .react-flow__edge.sso-placement-candidate .react-flow__edge-path']: {
          stroke: theme.palette.primary.main,
          strokeWidth: 3,
        },
        ['& .react-flow__edge.sso-placement-candidate']: {
          cursor: 'pointer',
        },
        ['& .react-flow__edge.sso-placement-dimmed']: {
          opacity: 0.2,
          pointerEvents: 'none',
        },
      })}
    >
      {existingFlowData?.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      <CompactStacksContext.Provider value={compactStacksContextValue}>
        <FlowCanvas
          resources={resources}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          mutateComponents={mutateComponents}
          onTemplateLoad={handleTemplateLoad}
          onWidgetLoad={handleWidgetLoad}
          onStepLoad={handleStepLoad}
          onResourceAdd={handleResourceAdd}
          onSave={existingFlowData?.isReadOnly ? undefined : handleSave}
          nodes={displayNodes}
          sourceNodes={nodes}
          sourceEdges={edges}
          edges={displayEdges}
          setNodes={setNodes}
          setEdges={setEdges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onEdgeClick={sso.handleEdgeClick}
          flowTitle={flowName}
          flowHandle={flowHandle}
          onFlowTitleChange={handleFlowNameChange}
          triggerAutoLayoutOnLoad={needsAutoLayout}
          resourcePanelFooter={ssoToggleFooter}
          onUndo={undo}
          onRedo={redo}
          canUndo={canUndo}
          canRedo={canRedo}
          isDirty={isDirty}
        />
      </CompactStacksContext.Provider>
      <SsoDisableConfirmDialog
        open={sso.isConfirmDialogOpen}
        checkpointCount={sso.ssoState.ssoCheckIds.length}
        onClose={sso.handleCloseConfirmDialog}
        onConfirm={sso.handleConfirmDisable}
      />
      <Snackbar open={sso.placement.active} anchorOrigin={{vertical: 'top', horizontal: 'center'}}>
        <Alert severity="info" sx={{width: '100%', alignItems: 'center'}}>
          <Stack direction="row" spacing={2} alignItems="center">
            <span>
              {t(
                'flows:sso.placementHint',
                'Click a highlighted connection to choose where the session checkpoint joins the flow.',
              )}
            </span>
            <GradientBorderButton size="small" onClick={sso.handleCancelPlacement}>
              {t('flows:sso.placementCancel', 'Cancel')}
            </GradientBorderButton>
          </Stack>
        </Alert>
      </Snackbar>
      <Snackbar
        open={errorSnackbar.open}
        autoHideDuration={6000}
        onClose={handleCloseErrorSnackbar}
        anchorOrigin={{vertical: 'bottom', horizontal: 'center'}}
      >
        <Alert onClose={handleCloseErrorSnackbar} severity="error" sx={{width: '100%'}}>
          {errorSnackbar.message}
        </Alert>
      </Snackbar>
      <Snackbar
        open={successSnackbar.open}
        autoHideDuration={6000}
        onClose={handleCloseSuccessSnackbar}
        anchorOrigin={{vertical: 'bottom', horizontal: 'center'}}
      >
        <Alert onClose={handleCloseSuccessSnackbar} severity="success" sx={{width: '100%'}}>
          {successSnackbar.message}
        </Alert>
      </Snackbar>
      <Snackbar
        open={infoSnackbar.open}
        autoHideDuration={8000}
        onClose={handleCloseInfoSnackbar}
        anchorOrigin={{vertical: 'top', horizontal: 'center'}}
      >
        <Alert
          onClose={handleCloseInfoSnackbar}
          severity="info"
          sx={{
            width: '100%',
            alignItems: 'center',
          }}
        >
          <Stack direction="row" spacing={2} alignItems="center">
            <span>{infoSnackbar.message}</span>
            <GradientBorderButton size="small" onClick={handleAutoLayoutClick}>
              {t('flows:core.canvas.buttons.autoLayout')}
            </GradientBorderButton>
          </Stack>
        </Alert>
      </Snackbar>
    </Box>
  );
}

export default FlowBuilder;
