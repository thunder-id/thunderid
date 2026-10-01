// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import type {Edge, Node} from '@xyflow/react';
import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import useCreateFlow from '../api/useCreateFlow';
import useUpdateFlow from '../api/useUpdateFlow';
import type {CreateFlowRequest, UpdateFlowRequest} from '../models/responses';
import {createFlowConfiguration, validateFlowGraph} from '../utils/reactFlowTransformer';

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
  /**
   * Called after the flow is persisted successfully, with the canvas data that
   * was actually saved. Saving is async and editing stays enabled meanwhile, so
   * dirty tracking must be rebased on this snapshot rather than on the live
   * graph at success time.
   */
  onSaved?: (savedCanvas: CanvasData) => void;
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

  // Resolves an error through the `flows` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `flows:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `flows:${key}`, options),
    [t],
  );

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
              onSaved?.(canvasData);
            },
            onError: (err: Error) => {
              showError(
                getErrorMessage(
                  err,
                  tForErrors,
                  'core.loginFlowBuilder.errors.saveFailed',
                  'Failed to save flow. Please try again.',
                ),
              );
            },
          },
        );
      } else {
        // Create new flow
        createFlow.mutate(flowConfig as CreateFlowRequest, {
          onSuccess: () => {
            showSuccess(t('flows:core.loginFlowBuilder.success.flowCreated'));
            onSaved?.(canvasData);
          },
          onError: (err: Error) => {
            showError(
              getErrorMessage(
                err,
                tForErrors,
                'core.loginFlowBuilder.errors.saveFailed',
                'Failed to save flow. Please try again.',
              ),
            );
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
      tForErrors,
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
