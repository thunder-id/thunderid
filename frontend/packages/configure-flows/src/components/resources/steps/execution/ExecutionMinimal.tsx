// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Card, IconButton, Tooltip} from '@wso2/oxygen-ui';
import {CogIcon, InfoIcon, TrashIcon} from '@wso2/oxygen-ui-icons-react';
import {Handle, Position, useNodeId, useReactFlow} from '@xyflow/react';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import ExecutionFactory from './execution-factory/ExecutionFactory';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import useInteractionState from '../../../../hooks/useInteractionState';
import useUIPanelState from '../../../../hooks/useUIPanelState';
import type {Step, StepData} from '../../../../models/steps';
import OutcomeHandleWrapper, {executionSurfaceMixin, nodeShadowMixin} from '../flowNodeStyles';
import StepTitle from '../StepTitle';

/**
 * Props interface of {@link ExecutionMinimal}
 */
export interface ExecutionMinimalPropsInterface {
  /**
   * Resource object of the execution step.
   */
  resource: Step;
}

/**
 * Execution (Minimal) Node component.
 *
 * @param props - Props injected to the component.
 * @returns Execution (Minimal) node component.
 */
function ExecutionMinimal({resource}: ExecutionMinimalPropsInterface): ReactElement {
  const {setLastInteractedResource, setLastInteractedStepId} = useInteractionState();
  const {setIsOpenResourcePropertiesPanel} = useUIPanelState();
  const {deleteElements} = useReactFlow();
  const stepId: string | null = useNodeId();

  const {t} = useTranslation();

  // Get the display label from resource.display.label, falling back to executor name
  const displayLabel = resource.display?.label ?? resource.data?.action?.executor?.name ?? 'Executor';
  const displayDescription = resource.display?.description;

  // Check if the node has action data with onSuccess/onFailure fields defined (even if empty)
  // This indicates the node supports branching and should show both handles
  const stepData = resource.data as StepData | undefined;
  const hasBranchingSupport = stepData?.action && 'onFailure' in stepData.action;
  const hasIncompleteSupport = stepData?.action && 'onIncomplete' in stepData.action;

  // Outcome handles can carry executor-specific labels (e.g. SSO-Check's Available/Unavailable);
  // fall back to the generic outcome labels otherwise.
  const outcomeLabels = resource.display?.outcomes;
  const successLabel = outcomeLabels?.success ?? t('flows:core.executions.handles.success');
  const failureLabel = outcomeLabels?.failure ?? t('flows:core.executions.handles.failure');
  const incompleteLabel = outcomeLabels?.incomplete ?? t('flows:core.executions.handles.incomplete');

  const handleConfigClick = (): void => {
    if (stepId !== null) {
      setLastInteractedStepId(stepId);
    }
    setLastInteractedResource({
      ...resource,
      config: {
        ...(resource?.config || {}),
        ...(typeof resource.data?.['config'] === 'object' && resource.data?.['config'] !== null
          ? resource.data['config']
          : {}),
      },
    });
    setIsOpenResourcePropertiesPanel(true);
  };

  return (
    <Box
      data-flow-node-surface
      data-testid="execution-minimal-step"
      sx={[{borderRadius: 1, position: 'relative'}, nodeShadowMixin]}
    >
      <Box
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        sx={{
          backgroundColor: '#151515',
          borderRadius: '8px 8px 0 0',
          px: 2,
          py: 1.25,
          height: 44,
          gap: 1.5,
        }}
      >
        <Box sx={{fontWeight: 500}}>
          <StepTitle label={displayLabel} />
        </Box>
        <Box display="flex" alignItems="center" gap={0.5}>
          {displayDescription && (
            <Tooltip title={displayDescription}>
              <Box
                display="inline-flex"
                alignItems="center"
                data-testid="execution-description-hint"
                sx={{color: 'common.white', cursor: 'help', px: 0.5}}
              >
                <InfoIcon size={18} />
              </Box>
            </Tooltip>
          )}
          <Tooltip title={t('flows:core.executions.tooltip.configurationHint')}>
            <IconButton
              size="small"
              onClick={handleConfigClick}
              sx={(theme) => ({
                color: 'common.white',
                '&:hover': {
                  ...theme.applyStyles('dark', {
                    backgroundColor: 'rgba(0, 0, 0, 0.2)',
                    color: 'common.white',
                  }),
                  ...theme.applyStyles('light', {
                    backgroundColor: 'rgba(0, 0, 0, 0.1)',
                    color: 'common.white',
                  }),
                },
              })}
            >
              <CogIcon size={18} />
            </IconButton>
          </Tooltip>
          {resource.deletable !== false && (
            <Tooltip title={t('flows:core.executions.tooltip.delete', 'Delete')}>
              <IconButton
                size="small"
                onClick={() => {
                  if (stepId) {
                    // eslint-disable-next-line @typescript-eslint/no-floating-promises
                    deleteElements({nodes: [{id: stepId}]});
                  }
                }}
                sx={(theme) => ({
                  color: 'common.white',
                  '&:hover': {
                    ...theme.applyStyles('dark', {
                      backgroundColor: 'rgba(0, 0, 0, 0.2)',
                      color: 'common.white',
                    }),
                    ...theme.applyStyles('light', {
                      backgroundColor: 'rgba(0, 0, 0, 0.1)',
                      color: 'common.white',
                    }),
                  },
                })}
              >
                <TrashIcon size={18} />
              </IconButton>
            </Tooltip>
          )}
        </Box>
      </Box>
      <Handle type="target" position={Position.Left} />
      <Card
        data-testid="execution-minimal-step-content"
        onClick={() => {
          setLastInteractedStepId(resource.id);
          setLastInteractedResource(resource);
        }}
        sx={[
          executionSurfaceMixin,
          {
            alignItems: 'center',
            border: 'none',
            borderRadius: '0 0 8px 8px',
            cursor: 'pointer',
            display: 'flex',
            flexFlow: 'column nowrap',
            justifyContent: 'center',
            minWidth: 200,
            p: 3,
            textAlign: 'left',
            ...(hasBranchingSupport && {minHeight: 60}),
          },
        ]}
      >
        <ExecutionFactory resource={resource} />
      </Card>
      {/* Success handle - always shown on the right */}
      {hasBranchingSupport ? (
        <Tooltip title={successLabel} placement="right">
          <OutcomeHandleWrapper kind="success" handleSize={12}>
            <Handle
              type="source"
              position={Position.Right}
              id={`${resource.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
              data-handle="execution-handle-success"
            />
          </OutcomeHandleWrapper>
        </Tooltip>
      ) : (
        <Handle
          type="source"
          position={Position.Right}
          id={`${resource.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        />
      )}
      {/* Failure handle - shown at the bottom when the action supports branching (has onFailure property) */}
      {hasBranchingSupport && (
        <Tooltip title={failureLabel} placement="bottom">
          <OutcomeHandleWrapper kind="failure" handleSize={12}>
            <Handle type="source" position={Position.Bottom} id="failure" data-handle="execution-handle-failure" />
          </OutcomeHandleWrapper>
        </Tooltip>
      )}
      {/* Incomplete handle - shown at the top when the action supports incomplete (has onIncomplete property) */}
      {hasIncompleteSupport && (
        <Tooltip title={incompleteLabel} placement="top">
          <OutcomeHandleWrapper kind="incomplete" handleSize={12}>
            <Handle
              type="source"
              position={Position.Top}
              id={`${resource.id}${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`}
              data-handle="execution-handle-incomplete"
            />
          </OutcomeHandleWrapper>
        </Tooltip>
      )}
    </Box>
  );
}

export default ExecutionMinimal;
