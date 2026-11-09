// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  GradientBorderButton,
  PageLoadingAnimation,
  QueryErrorNotice,
  ResourceDeployStatus,
} from '@thunderid/components';
import {useEmailProviders, useIdentityProviders, useSMSProviders} from '@thunderid/configure-connections';
import {useOrganizationAccess} from '@thunderid/configure-resource-sharing';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Snackbar, Stack} from '@wso2/oxygen-ui';
import type {Edge, Node, NodeChange} from '@xyflow/react';
import {useEdgesState, useNodesState, useUpdateNodeInternals} from '@xyflow/react';
import {type SetStateAction, useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate, useParams} from 'react-router';
import '@xyflow/react/dist/style.css';

import FlowCanvas from './FlowCanvas';
import useGetFlowBuilderResources from '../api/useGetFlowBuilderResources';
import useGetFlowById from '../api/useGetFlowById';
import {
  declaredVariablesOf,
  editsVariableRule,
  hasInvalidUnitDefaults,
  hasTermsForRemovedVariable,
} from '../components/flow-sharing/variableRules';
import SsoDisableConfirmDialog from '../components/SsoDisableConfirmDialog';
import SsoToggle from '../components/SsoToggle';
import FlowConstants from '../constants/FlowConstants';
import CompactStacksContext from '../context/CompactStacksContext';
import FlowTypeContext from '../context/FlowTypeContext';
import useEdgeGeneration from '../hooks/useEdgeGeneration';
import useElementAddition from '../hooks/useElementAddition';
import useFlowConfig from '../hooks/useFlowConfig';
import useFlowEvents from '../hooks/useFlowEvents';
import useFlowHistory, {computeGraphSignature} from '../hooks/useFlowHistory';
import useFlowInitialization from '../hooks/useFlowInitialization';
import useFlowNaming from '../hooks/useFlowNaming';
import useFlowRoutes from '../hooks/useFlowRoutes';
import useFlowSave from '../hooks/useFlowSave';
import useNodeTypes from '../hooks/useNodeTypes';
import useSnackbarNotifications from '../hooks/useSnackbarNotifications';
import useSsoToggle from '../hooks/useSsoToggle';
import useTemplateAndWidgetLoading from '../hooks/useTemplateAndWidgetLoading';
import useValidationStatus from '../hooks/useValidationStatus';
import {FlowType} from '../models/flows';
import type {CreateFlowRequest, FlowDefinitionResponse} from '../models/responses';
import {StepTypes, type StepData} from '../models/steps';
import {hasUnpositionedNodes} from '../utils/applyAutoLayout';
import collapseExecutorChains, {getExecutionStackHeadId} from '../utils/compactGraphTransforms';
import {mutateComponents} from '../utils/componentMutations';
import getConditionSkipEdges from '../utils/getConditionSkipEdges';
import {resolveConnectionProperty} from '../utils/resolveConnectionPlaceholders';
import {
  COMMON_GRAPH_VALIDATION_RULES,
  GRAPH_VALIDATION_RULES,
  SIGN_OUT_GRAPH_VALIDATION_RULES,
} from '../validation/validation-rules';

function FlowBuilder() {
  const {flowId: routeFlowId} = useParams<{flowId: string}>();
  const location = useLocation();
  const navigate = useNavigate();
  const flowRoutes = useFlowRoutes();
  const {can} = useCapabilitiesContext();
  const draftFlow = (location.state as {draftFlow?: CreateFlowRequest} | null)?.draftFlow;
  const isDraftFlow = routeFlowId === 'new' && Boolean(draftFlow);
  const flowId = isDraftFlow ? undefined : routeFlowId;
  const initialFlowData = useMemo<FlowDefinitionResponse | undefined>(() => {
    if (!isDraftFlow || !draftFlow) return undefined;
    // A draft in a project has no organization unit yet: the API places it in the system one on create.
    return {...draftFlow, ouId: draftFlow.ouId ?? '', id: '', activeVersion: 0, createdAt: '', updatedAt: ''};
  }, [draftFlow, isDraftFlow]);
  const [nodes, setNodes, defaultOnNodesChange] = useNodesState<Node>([]);
  const [edges, setStateEdges, onEdgesChange] = useEdgesState<Edge>([]);
  // getEdges() returns the display-only skip edges too; keep them out of the edges that are saved.
  const setEdges = useCallback(
    (update: SetStateAction<Edge[]>): void =>
      setStateEdges((current) => {
        const next = typeof update === 'function' ? update(current) : update;
        return next.some((edge) => edge.data?.['conditionSkip'] === true)
          ? next.filter((edge) => edge.data?.['conditionSkip'] !== true)
          : next;
      }),
    [setStateEdges],
  );
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
  const {
    data: existingFlowData,
    isLoading: isLoadingExistingFlow,
    error: flowLoadError,
    refetch: refetchFlow,
  } = useGetFlowById(flowId);

  // Determine if we're editing an existing flow
  const isEditingExistingFlow = Boolean(flowId && existingFlowData);

  // Inside a project, a flow of another project is not shown: the builder returns to the list.
  const {projectId} = useProjectScope();
  const isOtherProject = Boolean(projectId && existingFlowData && existingFlowData.projectId !== projectId);
  useEffect(() => {
    if (isOtherProject) void navigate(flowRoutes.flows.list(), {replace: true});
  }, [isOtherProject, navigate, flowRoutes]);

  // Resolves an error through the `flows` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `flows:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `flows:${key}`, options),
    [t],
  );

  // Flow naming hook
  const {flowName, flowHandle, needsAutoLayout, setNeedsAutoLayout, handleFlowNameChange} = useFlowNaming({
    existingFlowData: existingFlowData as {name?: string; handle?: string} | undefined,
    defaultName: initialFlowData?.name,
    defaultHandle: initialFlowData?.handle,
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
    initialFlowData,
    isLoadingExistingFlow,
    setNodes,
    setEdges,
    updateNodeInternals,
    generateEdges,
    validateEdges,
    edgeStyle,
    onNeedsAutoLayout: setNeedsAutoLayout,
  });

  // Auto-assign connections for executor nodes with placeholder IDP/sender IDs
  const {data: identityProviders, isPending: isIdentityProvidersPending} = useIdentityProviders();
  const {data: smsProviders, isPending: isSMSProvidersPending} = useSMSProviders();
  const {data: emailProviders, isPending: isEmailProvidersPending} = useEmailProviders();
  const hasAutoAssignedRef = useRef<boolean>(false);

  useEffect(() => {
    if (nodes.length === 0 || hasAutoAssignedRef.current) {
      return;
    }

    // Wait until every data source has settled. A source whose read failed has no data, and each
    // branch below skips its executors in that case, so one failed read cannot block the others.
    if (isIdentityProvidersPending || isSMSProvidersPending || isEmailProvidersPending) {
      return;
    }

    setNodes((currentNodes: Node[]) => {
      let changed = false;

      const updated = currentNodes.map((node: Node) => {
        if (node.type !== StepTypes.Execution) return node;

        const stepData = node.data as StepData | undefined;
        const executorName = (stepData?.action as {executor?: {name?: string}} | undefined)?.executor?.name;
        if (!executorName) return node;

        const resolved = resolveConnectionProperty(executorName, stepData?.properties, {
          identityProviders,
          smsProviders,
          emailProviders,
        });
        if (!resolved) return node;

        changed = true;
        return {
          ...node,
          data: {
            ...node.data,
            properties: {...(stepData?.properties ?? {}), ...resolved},
          },
        };
      });

      if (changed) {
        hasAutoAssignedRef.current = true;
        return updated;
      }

      return currentNodes;
    });
  }, [
    identityProviders,
    smsProviders,
    emailProviders,
    isIdentityProvidersPending,
    isSMSProvidersPending,
    isEmailProvidersPending,
    nodes.length,
    setNodes,
  ]);

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

  const flowType =
    (existingFlowData as {flowType?: FlowType} | undefined)?.flowType ??
    initialFlowData?.flowType ??
    FlowType.AUTHENTICATION;

  // SSO pairing is AUTHENTICATION-only and the confirm-action rule SIGNOUT-only.
  // Every flow type still gets the common rules.
  useEffect(() => {
    if (flowType === FlowType.AUTHENTICATION) {
      setGraphValidationRules(GRAPH_VALIDATION_RULES);
      return;
    }

    if (flowType === FlowType.SIGNOUT) {
      setGraphValidationRules(SIGN_OUT_GRAPH_VALIDATION_RULES);
      return;
    }

    setGraphValidationRules(COMMON_GRAPH_VALIDATION_RULES);
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
      isLoadingExistingFlow ||
      isIdentityProvidersPending ||
      isSMSProvidersPending ||
      isEmailProvidersPending ||
      hasUnpositionedNodes(nodes);
  }, [isLoadingExistingFlow, isIdentityProvidersPending, isSMSProvidersPending, isEmailProvidersPending, nodes]);

  const hasInteractedRef = useRef<boolean>(false);
  // How many interactions there have been, and how many there were when the last save was sent, so an
  // edit made while a save is in flight is never taken into the saved baseline.
  const interactionsRef = useRef<number>(0);
  const interactionsAtSaveRef = useRef<number>(0);
  useEffect(() => {
    const freezeBaseline = (): void => {
      interactionsRef.current += 1;
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

  // Only a flow that is saved, and so has an owner, can be shared.
  const sharedFlow = useMemo(
    () =>
      flowId && existingFlowData?.ouId
        ? {
            id: flowId,
            isReadOnly: existingFlowData.isReadOnly,
            name: existingFlowData.name,
            ouId: existingFlowData.ouId,
          }
        : undefined,
    [flowId, existingFlowData?.isReadOnly, existingFlowData?.name, existingFlowData?.ouId],
  );

  // The flow's organization access is held here, not in the Share view, so the flow's own Save saves it too, after the
  // flow: the server checks the terms against the stored flow, which has to hold the variables they name first.
  const sharingAccess = useOrganizationAccess({
    resourcePath: `flows/${sharedFlow?.id ?? ''}`,
    enabled: sharedFlow !== undefined,
    editsRule: editsVariableRule,
  });
  // The server refuses a default a unit starts with when it breaks the rules of its variable, so the editor does not
  // save over it. Saving the flow is held back too, as the sharing is saved with it.
  const hasInvalidSharing: boolean = useMemo(
    () =>
      sharingAccess.hasChanges &&
      !sharingAccess.isDeclared &&
      hasInvalidUnitDefaults(sharingAccess.draft, declaredVariablesOf(nodes)),
    [sharingAccess.draft, sharingAccess.hasChanges, sharingAccess.isDeclared, nodes],
  );
  const saveBlockedReason: string | undefined = hasInvalidSharing
    ? t(
        'flows:core.headerPanel.sharingInvalidTooltip',
        'Fix the organization unit defaults marked in Share before saving',
      )
    : undefined;
  const sharingAccessRef = useRef(sharingAccess);
  useEffect(() => {
    sharingAccessRef.current = sharingAccess;
  }, [sharingAccess]);

  const saveSharing = useCallback((): void => {
    sharingAccessRef.current
      .save()
      .then(() => showSuccess(t('flows:core.sharing.dialog.saved', 'Sharing updated.')))
      .catch((err: Error) =>
        showError(
          getErrorMessage(
            err,
            tForErrors,
            'core.sharing.dialog.saveError',
            'Failed to update how the flow is shared. Please try again.',
          ),
        ),
      );
  }, [showError, showSuccess, t, tForErrors]);

  // Rebase the clean baseline on the canvas snapshot that was actually
  // persisted — the user may have kept editing while the save request was in
  // flight, and those edits must stay dirty. History is intentionally NOT
  // reset here: undoing past a save point simply makes the flow dirty again.
  // A save is read back, which settles the graph once more (normalized fields, node internals); that
  // settle is taken as the clean baseline too, unless the user interacted after the save was sent or
  // interacts before it settles.
  const handleSaved = useCallback(
    (savedCanvas: {nodes: Node[]; edges: Edge[]}) => {
      hasInteractedRef.current = true;
      setSavedSignature(computeGraphSignature(savedCanvas.nodes, savedCanvas.edges));
      if (interactionsRef.current === interactionsAtSaveRef.current) setIsBaselineRebasePending(true);

      // The flow is stored now, so the sharing changes made beside it can be saved against it.
      const shared = sharingAccessRef.current;

      // A variable the flow no longer declares took its sharing terms with it on the server, so the policy held here is
      // out of date, with or without changes of its own. It is read again, and a draft built on it is dropped rather
      // than saved over the server's version.
      if (hasTermsForRemovedVariable(shared.savedPolicy, savedCanvas.nodes)) {
        const hadChanges: boolean = shared.hasChanges;

        shared.reset();

        if (hadChanges) {
          showError(
            t(
              'flows:core.sharing.dialog.termsDropped',
              'A variable you removed took its sharing terms with it, so your sharing changes were discarded. Share the flow again.',
            ),
          );
        }

        return;
      }

      if (shared.hasChanges) {
        saveSharing();
      }
    },
    [saveSharing, showError, t],
  );

  // Flow save hook
  const {handleSave} = useFlowSave({
    flowId,
    isEditingExistingFlow,
    isFlowValid,
    flowName,
    flowHandle,
    flowType,
    ouId: existingFlowData?.ouId ?? initialFlowData?.ouId,
    projectId: initialFlowData?.projectId,
    showError,
    showSuccess,
    setOpenValidationPanel,
    onSaved: handleSaved,
    onCreated: (createdFlowId) => {
      void navigate(flowRoutes.flows.detail(createdFlowId), {replace: true, state: null});
    },
  });
  const saveFlow = useCallback(
    (canvasData: Parameters<typeof handleSave>[0]): void => {
      interactionsAtSaveRef.current = interactionsRef.current;

      if (saveBlockedReason) {
        showError(saveBlockedReason);
        return;
      }

      // Only how the flow is shared changed, so there is nothing of the flow itself to save.
      if (!isDirty && sharingAccess.hasChanges) {
        saveSharing();
        return;
      }

      handleSave(canvasData);
    },
    [handleSave, isDirty, saveBlockedReason, saveSharing, sharingAccess.hasChanges, showError],
  );
  // Kept the same while the flow is, so the resource panel is not drawn again on every edit.
  const flowStatus = useMemo(
    () => (flowId ? <ResourceDeployStatus resourceType="flow" id={flowId} /> : undefined),
    [flowId],
  );

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

  const canvasEdges = useMemo(() => {
    const skipEdges = getConditionSkipEdges(nodes, t('flows:core.canvas.edges.onSkip', 'On skip'), edgeStyle);
    return skipEdges.length > 0 ? [...edges, ...skipEdges] : edges;
  }, [nodes, edges, t, edgeStyle]);

  const compactGraph = useMemo(
    () => (isVerboseMode ? null : collapseExecutorChains(nodes, canvasEdges, expandedStackIds)),
    [nodes, canvasEdges, isVerboseMode, expandedStackIds],
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
  const baseDisplayEdges = compactGraph?.edges ?? canvasEdges;

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

  if (isOtherProject) {
    return <PageLoadingAnimation />;
  }

  if (flowLoadError) {
    return (
      <QueryErrorNotice
        error={flowLoadError}
        t={tForErrors}
        variant="block"
        title={t('flows:core.loginFlowBuilder.errors.loadFailed', 'Failed to load flow')}
        onRetry={() => void refetchFlow()}
      />
    );
  }

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
      <FlowTypeContext.Provider value={flowType}>
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
            onSave={existingFlowData?.isReadOnly || !can('flow', flowId ? 'update' : 'create') ? undefined : saveFlow}
            nodes={displayNodes}
            sourceNodes={nodes}
            sourceEdges={edges}
            onDeleteRefused={showError}
            edges={displayEdges}
            setNodes={setNodes}
            setEdges={setEdges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onEdgeClick={sso.handleEdgeClick}
            flowTitle={flowName}
            flowStatus={flowStatus}
            flowHandle={flowHandle}
            onFlowTitleChange={handleFlowNameChange}
            triggerAutoLayoutOnLoad={needsAutoLayout}
            resourcePanelFooter={ssoToggleFooter}
            onUndo={undo}
            onRedo={redo}
            canUndo={canUndo}
            canRedo={canRedo}
            isDirty={isDirty || sharingAccess.hasChanges}
            sharing={sharedFlow}
            sharingAccess={sharingAccess}
            saveBlockedReason={saveBlockedReason}
          />
        </CompactStacksContext.Provider>
      </FlowTypeContext.Provider>
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
