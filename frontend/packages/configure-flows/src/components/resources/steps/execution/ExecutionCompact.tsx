// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Layers} from '@wso2/oxygen-ui-icons-react';
import {Handle, Position, useNodeId} from '@xyflow/react';
import {useContext, type KeyboardEvent, type MouseEvent, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import CompactStacksContext from '../../../../context/CompactStacksContext';
import useInteractionState from '../../../../hooks/useInteractionState';
import useUIPanelState from '../../../../hooks/useUIPanelState';
import type {Step, StepData} from '../../../../models/steps';
import ResourceDisplayImage from '../../../ResourceDisplayImage';
import OutcomeHandleWrapper, {
  executionSurfaceMixin,
  executionSurfaceScheme,
  mixWithPrimary,
  nodeShadowMixin,
} from '../flowNodeStyles';

/**
 * Props interface of {@link ExecutionCompact}
 */
export interface ExecutionCompactPropsInterface {
  /**
   * Resource object of the execution step.
   */
  resource: Step;
}

/**
 * Execution (Compact) Node component. Rendered in place of the full executor
 * card when the canvas is in compact (non-verbose) mode: an icon-only chip
 * that keeps the executor's branching handles on the canvas.
 *
 * @param props - Props injected to the component.
 * @returns Execution (Compact) node component.
 */
function ExecutionCompact({resource}: ExecutionCompactPropsInterface): ReactElement {
  const {setLastInteractedResource, setLastInteractedStepId} = useInteractionState();
  const {setIsOpenResourcePropertiesPanel} = useUIPanelState();
  const {collapseStack, expandedHeadIdToStackId} = useContext(CompactStacksContext);
  const stepId: string | null = useNodeId();

  const {t} = useTranslation();

  // The head chip of an expanded group offers restacking it.
  const restackableStackId = stepId ? expandedHeadIdToStackId.get(stepId) : undefined;

  const displayLabel = resource.display?.label ?? resource.data?.action?.executor?.name ?? 'Executor';
  const displayDescription = resource.display?.description;

  // Same branching detection as ExecutionMinimal: the presence of the
  // onFailure/onIncomplete fields (even empty) marks handle support.
  const stepData = resource.data as StepData | undefined;
  const hasBranchingSupport = stepData?.action && 'onFailure' in stepData.action;
  const hasIncompleteSupport = stepData?.action && 'onIncomplete' in stepData.action;

  const outcomeLabels = resource.display?.outcomes;
  const successLabel = outcomeLabels?.success ?? t('flows:core.executions.handles.success', 'Success');
  const failureLabel = outcomeLabels?.failure ?? t('flows:core.executions.handles.failure', 'Failure');
  const incompleteLabel = outcomeLabels?.incomplete ?? t('flows:core.executions.handles.incomplete', 'Incomplete');

  const handleOpenProperties = (): void => {
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
      data-testid="execution-compact-step"
      sx={{
        height: 48,
        position: 'relative',
        width: 48,
        // Scale the plain in/out handle dots down to chip proportions. The class
        // is doubled so this wins over the canvas-wide sizing rule.
        '& > .react-flow__handle.react-flow__handle': {borderWidth: 1, height: 8, width: 8},
      }}
    >
      <Tooltip
        placement="top"
        title={
          <Stack>
            <Typography variant="subtitle2">{displayLabel}</Typography>
            {displayDescription && <Typography variant="caption">{displayDescription}</Typography>}
          </Stack>
        }
      >
        <Box
          data-flow-node-surface
          data-testid="execution-compact-step-content"
          sx={[
            executionSurfaceMixin,
            nodeShadowMixin,
            {
              alignItems: 'center',
              borderRadius: '50%',
              cursor: 'pointer',
              display: 'flex',
              height: '100%',
              justifyContent: 'center',
              width: '100%',
              // Selection lives on React Flow's own node element, which is not an
              // ancestor in this component's tree.
              '.react-flow__node.selected &': {
                outline: '2px solid',
                outlineColor: 'primary.main',
                outlineOffset: '2px',
              },
            },
          ]}
          role="button"
          tabIndex={0}
          aria-label={displayLabel}
          onClick={handleOpenProperties}
          // A div with role="button" gets no native Enter/Space activation, and
          // in compact mode this chip is the only way into the executor.
          onKeyDown={(event: KeyboardEvent<HTMLElement>) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              handleOpenProperties();
            }
          }}
        >
          {resource.display?.image ? (
            <ResourceDisplayImage
              image={resource.display.image}
              label={displayLabel}
              size={24}
              preserveColor={resource.display?.preserveImageColor}
            />
          ) : (
            <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1}}>
              {displayLabel.charAt(0).toUpperCase()}
            </Typography>
          )}
        </Box>
      </Tooltip>
      {restackableStackId && (
        <Tooltip title={t('flows:core.executions.stack.restack', 'Restack executors')} placement="top">
          <IconButton
            size="small"
            data-testid="execution-compact-step-restack"
            sx={[
              (theme) =>
                executionSurfaceScheme(theme, (surface: string) => ({
                  backgroundColor: surface,
                  '&:hover': {backgroundColor: mixWithPrimary(theme, surface, 80)},
                })),
              {
                border: '2px solid',
                borderColor: 'background.default',
                color: 'text.primary',
                height: 22,
                position: 'absolute',
                right: -10,
                top: -10,
                width: 22,
                zIndex: 1,
              },
            ]}
            aria-label={t('flows:core.executions.stack.restack', 'Restack executors')}
            onClick={(event: MouseEvent<HTMLElement>) => {
              event.stopPropagation();
              collapseStack(restackableStackId);
            }}
          >
            <Layers size={12} />
          </IconButton>
        </Tooltip>
      )}
      <Handle type="target" position={Position.Left} />
      {/* Success handle - always shown on the right */}
      {hasBranchingSupport ? (
        <Tooltip title={successLabel} placement="right">
          <OutcomeHandleWrapper kind="success" handleSize={8}>
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
          <OutcomeHandleWrapper kind="failure" handleSize={8}>
            <Handle type="source" position={Position.Bottom} id="failure" data-handle="execution-handle-failure" />
          </OutcomeHandleWrapper>
        </Tooltip>
      )}
      {/* Incomplete handle - shown at the top when the action supports incomplete (has onIncomplete property) */}
      {hasIncompleteSupport && (
        <Tooltip title={incompleteLabel} placement="top">
          <OutcomeHandleWrapper kind="incomplete" handleSize={8}>
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

export default ExecutionCompact;
