// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography, type CSSObject, type Theme} from '@wso2/oxygen-ui';
import {Handle, Position, useNodeId, type NodeProps} from '@xyflow/react';
import {useContext, useMemo, type CSSProperties, type KeyboardEvent, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import CompactStacksContext from '../../../../context/CompactStacksContext';
import useValidationStatus from '../../../../hooks/useValidationStatus';
import type {Resource} from '../../../../models/resources';
import type {StepData} from '../../../../models/steps';
import {
  EXECUTION_STACK_MAX_LAYERS,
  type ExecutionStackData,
  type ExecutionStackMember,
} from '../../../../utils/compactGraphTransforms';
import ResourceDisplayImage from '../../../ResourceDisplayImage';
import ValidationErrorBoundary from '../../../validation-panel/ValidationErrorBoundary';
import {executionSurfaceMixin, executionSurfaceScheme, mixWithPrimary, nodeShadowMixin} from '../flowNodeStyles';

// Card-deck layers peeking out behind the chip, one per extra member (capped),
// suggesting the collapsed run at a glance. On hover the real member chips push
// out past them, so they fade away.
const stackLayerSurfaceSx = (theme: Theme): CSSObject =>
  executionSurfaceScheme(theme, (surface: string) => ({
    backgroundColor: `color-mix(in srgb, ${surface} 70%, transparent)`,
  }));

const stackLayerSx: CSSObject = {
  borderRadius: '50%',
  height: 48,
  position: 'absolute',
  top: 0,
  transition: 'opacity 0.2s ease',
  width: 48,
};

const stackFanChipSurfaceSx = (theme: Theme): CSSObject =>
  executionSurfaceScheme(theme, (surface: string) => ({
    backgroundColor: `color-mix(in srgb, ${surface} 92%, transparent)`,
  }));

// Hover preview: the stacked members stay stacked but each is pushed a bit
// further right from underneath the one above it, like cards nudged out of a
// deck. They sit behind the main chip (no z-index of their own, so the chip's
// z-index 1 keeps it on top) and only their right sliver shows. Pure decoration;
// pointer events stay off so neighboring nodes and edges remain interactive.
const stackFanChipSx: CSSObject = {
  alignItems: 'center',
  border: '2px solid',
  borderColor: 'background.default',
  borderRadius: '50%',
  color: 'text.primary',
  display: 'flex',
  height: 44,
  justifyContent: 'center',
  left: 2,
  opacity: 0,
  pointerEvents: 'none',
  position: 'absolute',
  top: 2,
  transform: 'translateX(0)',
  transition: 'transform 0.25s ease, opacity 0.2s ease',
  transitionDelay: 'calc(var(--fan-index) * 40ms)',
  width: 44,
};

interface MemberDisplay {
  description?: string;
  image?: string;
  label: string;
  preserveImageColor?: boolean;
}

const resolveMemberDisplay = (member: ExecutionStackMember): MemberDisplay => {
  const data = member.data as StepData | undefined;
  const display = data?.['display'] as MemberDisplay | undefined;
  const executorName = (data?.action as {executor?: {name?: string}} | undefined)?.executor?.name;
  return {
    description: display?.description,
    image: display?.image,
    label: display?.label ?? executorName ?? 'Executor',
    preserveImageColor: display?.preserveImageColor,
  };
};

/**
 * Execution (Stack) Node component. Rendered in compact (non-verbose) mode in
 * place of a run of consecutive non-branching executors: one chip showing the
 * first executor's icon with a "+N" count inside, a card-deck effect behind,
 * and a hover preview fanning the stacked members out to the right. Clicking
 * expands the stack into its individual member chips. A single in/out edge
 * anchor pair is kept.
 *
 * @param props - Props injected to the component.
 * @returns Execution (Stack) node component.
 */
function ExecutionStack({data}: NodeProps): ReactElement {
  const {expandStack} = useContext(CompactStacksContext);
  const {notifications} = useValidationStatus();
  const stackId: string | null = useNodeId();
  const {t} = useTranslation();

  const stackData = data as ExecutionStackData | undefined;
  const members = useMemo(() => stackData?.members ?? [], [stackData]);

  // Collapsing hides the members' own error boundaries, so the stack surfaces
  // the first member that has a notification. Expanding it then shows exactly
  // which executor is at fault.
  const notifiedMemberId = useMemo(
    () => members.find((member) => notifications.some((notification) => notification.hasResource(member.id)))?.id,
    [members, notifications],
  );
  const headMember = members.at(0);
  const headDisplay = headMember ? resolveMemberDisplay(headMember) : undefined;
  const stackedCount = Math.max(members.length - 1, 0);
  const layerCount = Math.min(stackedCount, EXECUTION_STACK_MAX_LAYERS);

  const handleExpand = (): void => {
    if (stackId) {
      expandStack(
        stackId,
        members.map((member) => member.id),
      );
    }
  };

  return (
    // A stack is wider than it is tall, so the boundary follows the rounded ends
    // of the deck with a pill radius rather than the default rounded rectangle.
    <ValidationErrorBoundary borderRadius="999px" resource={{id: notifiedMemberId ?? stackId ?? ''} as Resource}>
      <Box
        data-testid="execution-stack-step"
        sx={{
          height: 48,
          position: 'relative',
          // Scale the handle dots down to chip proportions. The class is doubled
          // so this wins over the canvas-wide sizing rule.
          '& .react-flow__handle.react-flow__handle': {borderWidth: 1, height: 8, width: 8},
        }}
      >
        <Box
          data-execution-stack-content
          sx={[
            (theme: Theme) =>
              executionSurfaceScheme(theme, (surface: string) => ({
                '&:hover [data-stack-chip]': {backgroundColor: mixWithPrimary(theme, surface, 92)},
              })),
            {
              cursor: 'pointer',
              height: 48,
              position: 'relative',
              '&:hover [data-stack-layer]': {opacity: 0},
              // 32px per step is the smallest push that clears the icon of the
              // chip above it, so every icon stays centered in its own circle and
              // fully visible while the circles still overlap enough to read as a
              // stack.
              '&:hover [data-stack-fan-chip]': {
                opacity: 1,
                transform: 'translateX(calc((var(--fan-index) + 1) * 32px))',
              },
            },
          ]}
          role="button"
          tabIndex={0}
          aria-label={members.map((member) => resolveMemberDisplay(member).label).join(', ')}
          aria-expanded={false}
          title={t('flows:core.executions.stack.expandHint', 'Click to expand')}
          onClick={handleExpand}
          // A div with role="button" gets no native Enter/Space activation, so
          // the stack would be focusable but impossible to expand by keyboard.
          onKeyDown={(event: KeyboardEvent<HTMLElement>) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              handleExpand();
            }
          }}
        >
          {Array.from({length: layerCount}, (_, index) => (
            <Box key={index} data-stack-layer sx={[stackLayerSurfaceSx, stackLayerSx, {left: (index + 1) * 4}]} />
          ))}
          {/* Hover preview: the stacked members slide out of the deck to the
            right. The main chip itself stays put; only its color changes. */}
          {members.slice(1).map((member, index) => {
            const display = resolveMemberDisplay(member);
            return (
              <Box
                key={member.id}
                data-stack-fan-chip
                sx={[stackFanChipSurfaceSx, stackFanChipSx]}
                style={{'--fan-index': index} as CSSProperties}
              >
                {display.image ? (
                  <ResourceDisplayImage
                    image={display.image}
                    label={display.label}
                    size={16}
                    preserveColor={display.preserveImageColor}
                  />
                ) : (
                  <Typography variant="caption" sx={{fontWeight: 600, lineHeight: 1}}>
                    {display.label.charAt(0).toUpperCase()}
                  </Typography>
                )}
              </Box>
            );
          })}
          <Box
            data-flow-node-surface
            data-stack-chip
            sx={[
              executionSurfaceMixin,
              nodeShadowMixin,
              {
                alignItems: 'center',
                border: '2px solid',
                borderColor: 'background.default',
                borderRadius: '50%',
                display: 'flex',
                flexDirection: 'column',
                gap: '1px',
                height: 48,
                justifyContent: 'center',
                position: 'relative',
                width: 48,
                zIndex: 1,
                // Selection lives on React Flow's own node element, which is not
                // an ancestor in this component's tree.
                '.react-flow__node.selected &': {
                  outline: '2px solid',
                  outlineColor: 'primary.main',
                  outlineOffset: '2px',
                },
              },
            ]}
          >
            {headDisplay?.image ? (
              <ResourceDisplayImage
                image={headDisplay.image}
                label={headDisplay.label}
                size={stackedCount > 0 ? 18 : 24}
                preserveColor={headDisplay.preserveImageColor}
              />
            ) : (
              <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1}}>
                {(headDisplay?.label ?? 'Executor').charAt(0).toUpperCase()}
              </Typography>
            )}
            {/* The "+N" count sits inside the circle, under the first executor's icon. */}
            {stackedCount > 0 && (
              <Typography
                variant="caption"
                data-testid="execution-stack-badge"
                sx={{fontSize: '10px', fontWeight: 600, lineHeight: 1, opacity: 0.85}}
              >
                +{stackedCount}
              </Typography>
            )}
          </Box>
        </Box>
        <Handle type="target" position={Position.Left} />
        <Handle
          type="source"
          position={Position.Right}
          id={`${stackId ?? ''}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        />
      </Box>
    </ValidationErrorBoundary>
  );
}

export default ExecutionStack;
