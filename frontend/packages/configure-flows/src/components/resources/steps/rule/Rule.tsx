// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Tooltip, Typography} from '@wso2/oxygen-ui';
import {CrossIcon} from '@wso2/oxygen-ui-icons-react';
import {Handle, Position, useNodeId, useReactFlow} from '@xyflow/react';
import {memo, useCallback, useMemo, useRef, type DragEvent, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useInteractionState from '../../../../hooks/useInteractionState';
import type {Resource} from '../../../../models/resources';
import type {CommonStepFactoryPropsInterface} from '../CommonStepFactory';
import {executionSurfaceMixin, nodeShadowMixin} from '../flowNodeStyles';

/**
 * Props interface of {@link Rule}
 */
export type RulePropsInterface = CommonStepFactoryPropsInterface;

/**
 * Representation of an empty step in the flow builder.
 *
 * @param props - Props injected to the component.
 * @returns Rule component.
 */
function Rule({data, id}: RulePropsInterface): ReactElement {
  const {t} = useTranslation();
  const nodeId: string | null = useNodeId();
  // The `data` and `id` props already contain the node's data, passed down from React Flow
  const {deleteElements} = useReactFlow();
  const {setLastInteractedResource} = useInteractionState();

  const ref = useRef<HTMLDivElement>(null);

  const handleDragOver: (event: DragEvent<HTMLDivElement>) => void = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    const {dataTransfer} = event;
    if (dataTransfer) {
      dataTransfer.dropEffect = 'move';
    }
  }, []);

  const handleDrop: (e: DragEvent<HTMLDivElement>) => void = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
  }, []);

  // Memoize ruleStep to prevent recreation on each render
  const ruleStep: Resource = useMemo(
    () =>
      ({
        ...(typeof data === 'object' && data !== null ? data : {}),
        id: id ?? nodeId ?? '',
      }) as Resource,
    [data, id, nodeId],
  );

  return (
    <Box
      ref={ref}
      data-flow-node-surface
      data-testid="rule-node"
      onDrop={handleDrop}
      onDrag={handleDragOver}
      sx={[
        executionSurfaceMixin,
        nodeShadowMixin,
        {
          border: '1px solid',
          borderColor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.45)',
          borderRadius: 1,
        },
      ]}
    >
      <Handle type="target" position={Position.Left} />
      <Box
        display="flex"
        justifyContent="space-between"
        data-testid="rule-action-panel"
        sx={{p: 2.5}}
        onClick={() => setLastInteractedResource(ruleStep)}
      >
        <Typography variant="body2" sx={{color: 'primary.main'}}>
          {t('flows:core.rule.conditionalRule')}
        </Typography>
        <Tooltip title={t('flows:core.rule.remove')}>
          <IconButton
            size="small"
            onClick={() => {
              if (nodeId) {
                // eslint-disable-next-line @typescript-eslint/no-floating-promises
                deleteElements({nodes: [{id: nodeId}]});
              }
            }}
            sx={{
              border: '2px solid',
              borderColor: 'grey.500',
              color: 'grey.500',
              ml: 2.5,
              p: '2px',
              '&:hover': {borderColor: 'text.primary', color: 'text.primary'},
            }}
          >
            <CrossIcon />
          </IconButton>
        </Tooltip>
      </Box>
      <Handle type="source" position={Position.Right} id="a" />
    </Box>
  );
}

// Memoize Rule to prevent re-renders when parent re-renders with same props
const MemoizedRule = memo(Rule, (prevProps, nextProps) => {
  // Re-render if data changed
  if (prevProps.data !== nextProps.data) {
    return false;
  }
  // Re-render if id changed
  if (prevProps.id !== nextProps.id) {
    return false;
  }
  return true;
});

export default MemoizedRule;
