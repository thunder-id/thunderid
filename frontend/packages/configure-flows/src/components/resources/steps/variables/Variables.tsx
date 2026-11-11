// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {alpha, Box, Card, IconButton, Tooltip, Typography, type Theme} from '@wso2/oxygen-ui';
import {CogIcon, TrashIcon, Variable as VariableIcon} from '@wso2/oxygen-ui-icons-react';
import {Handle, Position, useReactFlow} from '@xyflow/react';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import useInteractionState from '../../../../hooks/useInteractionState';
import useUIPanelState from '../../../../hooks/useUIPanelState';
import {ResourceTypes} from '../../../../models/resources';
import {StaticStepTypes, StepCategories, type Step, type StepData} from '../../../../models/steps';
import OutcomeHandleWrapper, {executionSurfaceMixin, nodeShadowMixin} from '../flowNodeStyles';
import StepTitle from '../StepTitle';

/**
 * Props interface of {@link Variables}
 */
export interface VariablesPropsInterface {
  /**
   * The data of the node. Its `inputs` are the variables the flow declares.
   */
  data?: StepData;
  /**
   * The id of the node, which its source handle is named after.
   */
  id: string;
}

const VARIABLES_NODE_WIDTH = 260;

/**
 * Variables Node component. It declares the variables a flow reads and resolves them for the organization unit the
 * flow runs for. It is drawn like an executor, in purple, and its configuration opens the variables editor.
 *
 * @param props - Props injected to the component.
 * @returns Variables node component.
 */
function Variables({data = {}, id}: VariablesPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {setLastInteractedResource, setLastInteractedStepId} = useInteractionState();
  const {setIsOpenResourcePropertiesPanel} = useUIPanelState();
  const {deleteElements} = useReactFlow();

  const declared: unknown[] = Array.isArray(data['inputs']) ? (data['inputs'] as unknown[]) : [];
  const label: string = t('flows:core.variables.label', 'Resolve Variables');

  const resource: Step = {
    category: StepCategories.Workflow,
    data: {...data, displayOnly: true},
    deletable: true,
    display: {label, showOnResourcePanel: false},
    id,
    resourceType: ResourceTypes.Step,
    type: StaticStepTypes.Variables,
  } as unknown as Step;

  const select = (): void => {
    setLastInteractedStepId(id);
    setLastInteractedResource(resource);
  };

  const configure = (): void => {
    select();
    setIsOpenResourcePropertiesPanel(true);
  };

  const remove = (): void => {
    // eslint-disable-next-line @typescript-eslint/no-floating-promises
    deleteElements({nodes: [{id}]});
  };

  return (
    <Box
      data-flow-node-surface
      data-testid="variables-node"
      sx={[{borderRadius: 1, position: 'relative', width: VARIABLES_NODE_WIDTH}, nodeShadowMixin]}
    >
      <Box
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        sx={{
          backgroundColor: 'primary.main',
          borderBottomLeftRadius: 0,
          borderBottomRightRadius: 0,
          borderRadius: 1,
          color: 'primary.contrastText',
          gap: 1.5,
          height: 44,
          px: 2,
          py: 1.25,
        }}
      >
        <Box sx={{fontWeight: 500}}>
          <StepTitle label={label} />
        </Box>
        <Box display="flex" alignItems="center" gap={0.5}>
          <Tooltip title={t('flows:core.variables.tooltip.configure', 'Configure variables')}>
            <IconButton
              aria-label={t('flows:core.variables.tooltip.configure', 'Configure variables')}
              data-testid="variables-node-configure"
              size="small"
              onClick={configure}
              sx={{
                color: 'primary.contrastText',
                '&:hover': {backgroundColor: (theme: Theme) => alpha(theme.palette.common.black, 0.15)},
              }}
            >
              <CogIcon size={18} />
            </IconButton>
          </Tooltip>
          <Tooltip title={t('common:actions.delete', 'Delete')}>
            <IconButton
              aria-label={t('common:actions.delete', 'Delete')}
              data-testid="variables-node-delete"
              size="small"
              onClick={remove}
              sx={{
                color: 'primary.contrastText',
                '&:hover': {backgroundColor: (theme: Theme) => alpha(theme.palette.common.black, 0.15)},
              }}
            >
              <TrashIcon size={18} />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>
      <Handle type="target" position={Position.Left} />
      <Card
        data-testid="variables-node-content"
        onClick={select}
        sx={[
          executionSurfaceMixin,
          {
            border: 'none',
            borderRadius: 1,
            borderTopLeftRadius: 0,
            borderTopRightRadius: 0,
            cursor: 'pointer',
            p: 2.5,
            textAlign: 'left',
          },
        ]}
      >
        <Box display="flex" alignItems="center" gap={1.5}>
          <Box
            sx={{
              alignItems: 'center',
              backgroundColor: (theme: Theme) => alpha(theme.palette.primary.main, 0.14),
              borderRadius: 1,
              color: 'primary.main',
              display: 'inline-flex',
              flexShrink: 0,
              height: 32,
              justifyContent: 'center',
              width: 32,
            }}
          >
            <VariableIcon size={18} />
          </Box>
          <Box sx={{minWidth: 0}}>
            <Typography variant="body1">{label}</Typography>
            <Typography variant="caption" color="text.secondary" data-testid="variables-node-count">
              {t('flows:core.variables.count', '{{count}} variables', {count: declared.length})}
            </Typography>
          </Box>
        </Box>
      </Card>
      <Handle
        id={`${id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
        type="source"
        position={Position.Right}
      />
      <Tooltip title={t('flows:core.executions.handles.failure', 'Failure')} placement="bottom">
        <OutcomeHandleWrapper kind="failure" handleSize={8}>
          <Handle type="source" position={Position.Bottom} id="failure" data-testid="variables-node-handle-failure" />
        </OutcomeHandleWrapper>
      </Tooltip>
    </Box>
  );
}

export default Variables;
