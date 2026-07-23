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

import type {Edge, Node} from '@xyflow/react';
import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import useCreateFlow from '@/features/flows/api/useCreateFlow';
import useUpdateFlow from '@/features/flows/api/useUpdateFlow';
import type {CreateFlowRequest, UpdateFlowRequest} from '@/features/flows/models/responses';
import {createFlowConfiguration, validateFlowGraph} from '@/features/flows/utils/reactFlowTransformer';

/**
 * Canvas data from React Flow.
 */
export interface CanvasData {
  /** Nodes in the flow. */
  nodes: Node[];
  /** Edges in the flow. */
  edges: Edge[];
  /** Viewport position and zoom. */
  viewport: {x: number; y: number; zoom: number};
}

/**
 * Props for the useFlowSave hook.
 */
export interface UseFlowSaveProps {
  /** Flow ID if editing an existing flow. */
  flowId?: string;
  /** Whether we're editing an existing flow. */
  isEditingExistingFlow: boolean;
  /** Whether the flow is valid. */
  isFlowValid: boolean;
  /** Flow name. */
  flowName: string;
  /** Flow handle. */
  flowHandle: string;
  /** Flow type. */
  flowType: string;
  /** Callback to show error notification. */
  showError: (message: string) => void;
  /** Callback to show success notification. */
  showSuccess: (message: string) => void;
  /** Callback to open the validation panel. */
  setOpenValidationPanel?: (open: boolean) => void;
  /** Called after the flow is persisted successfully (to clear the dirty state). */
  onSaved?: () => void;
}

/**
 * Return type for the useFlowSave hook.
 */
export interface UseFlowSaveReturn {
  /** Handle save button click. */
  handleSave: (canvasData: CanvasData) => void;
  /** Whether a save operation is in progress. */
  isSaving: boolean;
}

/**
 * Hook to handle flow save logic including validation and API calls.
 *
 * @param props - Configuration options for the hook.
 * @returns Save handler and save state.
 */
const useFlowSave = (props: UseFlowSaveProps): UseFlowSaveReturn => {
  const {
    flowId,
    isEditingExistingFlow,
    isFlowValid,
    flowName,
    flowHandle,
    flowType,
    showError,
    showSuccess,
    setOpenValidationPanel,
    onSaved,
  } = props;

  const {t} = useTranslation();
  const createFlow = useCreateFlow();
  const updateFlow = useUpdateFlow();

  /**
   * Handle save button click - transforms React Flow data to backend format.
   */
  const handleSave = useCallback(
    (canvasData: CanvasData) => {
      // Check if there are validation errors in the validation panel
      if (!isFlowValid) {
        showError(t('flows:core.loginFlowBuilder.errors.validationRequired'));
        setOpenValidationPanel?.(true);
        return;
      }

      const flowConfig = createFlowConfiguration(canvasData, flowName, flowHandle, flowType);
      const errors = validateFlowGraph({nodes: flowConfig.nodes});

      if (errors.length > 0) {
        showError(t('flows:core.loginFlowBuilder.errors.structureValidationFailed', {error: errors[0]}));
        return;
      }

      // Send to backend API - use update if editing existing flow, create if new
      if (isEditingExistingFlow && flowId) {
        // Update existing flow
        updateFlow.mutate(
          {
            flowId,
            flowData: flowConfig as UpdateFlowRequest,
          },
          {
            onSuccess: () => {
              showSuccess(t('flows:core.loginFlowBuilder.success.flowUpdated'));
              onSaved?.();
            },
            onError: () => {
              showError(t('flows:core.loginFlowBuilder.errors.saveFailed'));
            },
          },
        );
      } else {
        // Create new flow
        createFlow.mutate(flowConfig as CreateFlowRequest, {
          onSuccess: () => {
            showSuccess(t('flows:core.loginFlowBuilder.success.flowCreated'));
            onSaved?.();
          },
          onError: () => {
            showError(t('flows:core.loginFlowBuilder.errors.saveFailed'));
          },
        });
      }
    },
    [
      isFlowValid,
      isEditingExistingFlow,
      flowId,
      flowName,
      flowHandle,
      flowType,
      showError,
      showSuccess,
      setOpenValidationPanel,
      onSaved,
      t,
      createFlow,
      updateFlow,
    ],
  );

  return {
    handleSave,
    isSaving: createFlow.isPending || updateFlow.isPending,
  };
};

export default useFlowSave;
