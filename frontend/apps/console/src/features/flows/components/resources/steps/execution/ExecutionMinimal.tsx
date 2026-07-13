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

import {Box, Card, IconButton, Tooltip, Typography} from '@wso2/oxygen-ui';
import {CogIcon, TrashIcon} from '@wso2/oxygen-ui-icons-react';
import {Handle, Position, useNodeId, useReactFlow} from '@xyflow/react';
import classNames from 'classnames';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import ExecutionFactory from './execution-factory/ExecutionFactory';
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import useInteractionState from '@/features/flows/hooks/useInteractionState';
import useUIPanelState from '@/features/flows/hooks/useUIPanelState';
import type {Step, StepData} from '@/features/flows/models/steps';
import './ExecutionMinimal.scss';

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
        ...(typeof resource.data?.config === 'object' && resource.data?.config !== null ? resource.data.config : {}),
      },
    });
    setIsOpenResourcePropertiesPanel(true);
  };

  return (
    <Box className={classNames('execution-minimal-step', {'has-branching': hasBranchingSupport})}>
      <Box
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        className="execution-minimal-step-action-panel"
        sx={{
          backgroundColor: 'secondary.main',
          px: 2,
          py: 1.25,
          height: 44,
        }}
      >
        <Typography
          variant="body2"
          className="execution-minimal-step-title"
          sx={{
            color: 'common.white',
            fontWeight: 500,
          }}
        >
          {displayLabel}
        </Typography>
        <Box display="flex" alignItems="center" gap={0.5}>
          <Tooltip title={t('flows:core.executions.tooltip.configurationHint')}>
            <IconButton
              size="small"
              onClick={handleConfigClick}
              className="execution-minimal-step-action"
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
                className="execution-minimal-step-action"
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
        className="execution-minimal-step-content"
        onClick={() => {
          setLastInteractedStepId(resource.id);
          setLastInteractedResource(resource);
        }}
      >
        <ExecutionFactory resource={resource} />
      </Card>
      {/* Success handle - always shown on the right */}
      {hasBranchingSupport ? (
        <Tooltip title={successLabel} placement="right">
          <Box className="handle-wrapper success-wrapper">
            <Handle
              type="source"
              position={Position.Right}
              id={`${resource.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
              className="execution-handle-success"
            />
          </Box>
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
          <Box className="handle-wrapper failure-wrapper">
            <Handle type="source" position={Position.Bottom} id="failure" className="execution-handle-failure" />
          </Box>
        </Tooltip>
      )}
      {/* Incomplete handle - shown at the top when the action supports incomplete (has onIncomplete property) */}
      {hasIncompleteSupport && (
        <Tooltip title={incompleteLabel} placement="top">
          <Box className="handle-wrapper incomplete-wrapper">
            <Handle
              type="source"
              position={Position.Top}
              id={`${resource.id}${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`}
              className="execution-handle-incomplete"
            />
          </Box>
        </Tooltip>
      )}
    </Box>
  );
}

export default ExecutionMinimal;
