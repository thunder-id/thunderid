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
import type {Edge, Node} from '@xyflow/react';
import {useEdgesState, useNodesState, useUpdateNodeInternals} from '@xyflow/react';
import {useCallback, useEffect, useMemo, useRef} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import '@xyflow/react/dist/style.css';
import useGetLoginFlowBuilderResources from '../api/useGetLoginFlowBuilderResources';
import {EXECUTOR_TO_IDP_TYPE_MAP} from '../components/resource-property-panel/extended-properties/execution-properties/constants';
import LoginFlowConstants from '../constants/LoginFlowConstants';
import useEdgeGeneration from '../hooks/useEdgeGeneration';
import useElementAddition from '../hooks/useElementAddition';
import useFlowInitialization from '../hooks/useFlowInitialization';
import useFlowNaming from '../hooks/useFlowNaming';
import useFlowSave from '../hooks/useFlowSave';
import useNodeTypes from '../hooks/useNodeTypes';
import useSnackbarNotifications from '../hooks/useSnackbarNotifications';
import useTemplateAndWidgetLoading from '../hooks/useTemplateAndWidgetLoading';
import {mutateComponents} from '../utils/componentMutations';
import GradientBorderButton from '@/features/applications/components/GradientBorderButton';
import useGetFlowById from '@/features/flows/api/useGetFlowById';
import FlowBuilder from '@/features/flows/components/FlowBuilder';
import useFlowConfig from '@/features/flows/hooks/useFlowConfig';
import useFlowEvents from '@/features/flows/hooks/useFlowEvents';
import useValidationStatus from '@/features/flows/hooks/useValidationStatus';
import {ExecutionTypes, StepTypes, type StepData} from '@/features/flows/models/steps';

const SMS_EXECUTORS = new Set<string>([ExecutionTypes.SMSExecutor]);

function LoginFlowBuilder() {
  const {flowId} = useParams<{flowId: string}>();
  const [nodes, setNodes, defaultOnNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const {t} = useTranslation();

  const {data: resources} = useGetLoginFlowBuilderResources();
  const {edgeStyle, isVerboseMode} = useFlowConfig();
  const {triggerAutoLayout, onRestoreFromHistory, onElementAdded} = useFlowEvents();
  const {isValid: isFlowValid, setOpenValidationPanel} = useValidationStatus();
  const updateNodeInternals = useUpdateNodeInternals();

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
    startStepId: LoginFlowConstants.START_STEP_ID,
    endStepId: LoginFlowConstants.END_STEP_ID,
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

  // Auto-assign connections for executor nodes with placeholder IDP/sender IDs
  const {data: identityProviders} = useIdentityProviders();
  const {data: smsProviders} = useSMSProviders();
  const hasAutoAssignedRef = useRef<boolean>(false);

  useEffect(() => {
    if (nodes.length === 0 || hasAutoAssignedRef.current) {
      return;
    }

    // Wait until both data sources are available
    if (!identityProviders || !smsProviders) {
      return;
    }

    setNodes((currentNodes: Node[]) => {
      let changed = false;

      const updated = currentNodes.map((node: Node) => {
        if (node.type !== StepTypes.Execution) return node;

        const stepData = node.data as StepData | undefined;
        const executorName = (stepData?.action as {executor?: {name?: string}} | undefined)?.executor?.name;
        if (!executorName) return node;

        const {senderId: currentSenderId = '', idpId: currentIdpId = ''} =
          (stepData?.properties as Record<string, string> | undefined) ?? {};

        // Handle SMS executors - auto-assign senderId
        if (SMS_EXECUTORS.has(executorName) && smsProviders) {
          if (currentSenderId === '{{SENDER_ID}}' || currentSenderId === '') {
            if (smsProviders.length === 1) {
              changed = true;
              return {
                ...node,
                data: {
                  ...node.data,
                  properties: {
                    ...((stepData?.properties as Record<string, unknown>) ?? {}),
                    senderId: smsProviders[0].id,
                  },
                },
              };
            }
          }
          return node;
        }

        // Handle IDP executors - auto-assign idpId
        const idpType = EXECUTOR_TO_IDP_TYPE_MAP[executorName];
        if (!idpType || !identityProviders) return node;

        if (currentIdpId !== '{{IDP_ID}}' && currentIdpId !== '') return node;

        const matching = identityProviders.filter((idp) => idp.type === idpType);
        if (matching.length !== 1) return node;

        changed = true;
        return {
          ...node,
          data: {
            ...node.data,
            properties: {...((stepData?.properties as Record<string, unknown>) ?? {}), idpId: matching[0].id},
          },
        };
      });

      if (changed) {
        hasAutoAssignedRef.current = true;
        return updated;
      }

      return currentNodes;
    });
  }, [identityProviders, smsProviders, nodes.length, setNodes]);

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

  // Flow save hook
  const {handleSave} = useFlowSave({
    flowId,
    isEditingExistingFlow,
    isFlowValid,
    flowName,
    flowHandle,
    flowType: (existingFlowData as {flowType?: string} | undefined)?.flowType ?? 'AUTHENTICATION',
    showError,
    showSuccess,
    setOpenValidationPanel,
  });

  const onNodesChange = defaultOnNodesChange;

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

  // Filter nodes and edges based on verbose mode
  const filteredNodes = useMemo(() => {
    if (isVerboseMode) {
      return nodes;
    }
    // Hide execution nodes in non-verbose mode
    return nodes.filter((node) => node.type !== StepTypes.Execution);
  }, [nodes, isVerboseMode]);

  const filteredEdges = useMemo(() => {
    if (isVerboseMode) {
      return edges;
    }
    // Hide edges connected to execution nodes in non-verbose mode
    const executionNodeIds = new Set(nodes.filter((node) => node.type === StepTypes.Execution).map((node) => node.id));
    return edges.filter((edge) => !executionNodeIds.has(edge.source) && !executionNodeIds.has(edge.target));
  }, [edges, nodes, isVerboseMode]);

  return (
    <Box
      sx={{
        width: '100%',
      }}
    >
      {existingFlowData?.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      <FlowBuilder
        resources={resources}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        mutateComponents={mutateComponents}
        onTemplateLoad={handleTemplateLoad}
        onWidgetLoad={handleWidgetLoad}
        onStepLoad={handleStepLoad}
        onResourceAdd={handleResourceAdd}
        onSave={existingFlowData?.isReadOnly ? undefined : handleSave}
        nodes={filteredNodes}
        edges={filteredEdges}
        setNodes={setNodes}
        setEdges={setEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        flowTitle={flowName}
        flowHandle={flowHandle}
        onFlowTitleChange={handleFlowNameChange}
        triggerAutoLayoutOnLoad={needsAutoLayout}
      />
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

export default LoginFlowBuilder;
