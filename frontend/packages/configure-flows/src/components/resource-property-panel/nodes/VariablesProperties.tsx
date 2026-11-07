// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getFlowVariableOptions} from '@thunderid/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import {Box, Button, FormLabel, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronRight, Eye, Plus, TrashIcon} from '@wso2/oxygen-ui-icons-react';
import {useContext, useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import FlowVariablesContext from '../../../context/FlowVariablesContext';
import useFlowVariableConnections from '../../variables-dialog/useFlowVariableConnections';
import {createVariable} from '../../variables-dialog/variableDefinitions';
import VariableEditor from '../../variables-dialog/VariableEditor';
import VariablesPreviewDialog from '../../variables-dialog/VariablesPreviewDialog';
import type {CommonResourcePropertiesPropsInterface} from '../CommonResourceProperties';

export type VariablesPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

/**
 * Properties of the variables node. The variables of the node are set up here directly, and a preview shows how a
 * delegated admin sees them.
 *
 * @param props - Props injected to the component.
 * @returns Variables properties component.
 */
function VariablesProperties({resource, onChange}: VariablesPropertiesPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {groups, setGroups} = useContext(FlowVariablesContext);
  const connections = useFlowVariableConnections();
  const [selected, setSelected] = useState<number | null>(null);
  const [isPreviewOpen, setIsPreviewOpen] = useState<boolean>(false);

  const {inputs} = (resource.data ?? {}) as {inputs?: unknown};
  const own: FlowVariableDefinition[] = Array.isArray(inputs) ? (inputs as FlowVariableDefinition[]) : [];

  // The identifiers must be unique across every variables node of the flow, so the editor checks against all of them.
  const isInFlow: boolean = groups.some((group) => group.nodeId === resource.id);
  const variables: FlowVariableDefinition[] = isInFlow
    ? (groups.find((group) => group.nodeId === resource.id)?.variables ?? [])
    : own;
  const all: FlowVariableDefinition[] = isInFlow ? groups.flatMap((group) => group.variables) : own;
  const offset: number = isInFlow
    ? groups
        .slice(
          0,
          groups.findIndex((group) => group.nodeId === resource.id),
        )
        .reduce((total, group) => total + group.variables.length, 0)
    : 0;

  const index: number = selected === null ? -1 : Math.min(selected, variables.length - 1);

  const save = (next: FlowVariableDefinition[]): void => {
    if (isInFlow) {
      setGroups([{nodeId: resource.id, variables: next}]);
    } else {
      onChange('data.inputs', next, resource);
    }
  };

  const add = (): void => {
    save([...variables, createVariable(all)]);
    setSelected(variables.length);
  };

  const remove = (position: number): void => {
    save(variables.filter((_: FlowVariableDefinition, i: number) => i !== position));
    if (index === -1) return;
    if (position === index) setSelected(null);
    else setSelected(position < index ? index - 1 : index);
  };

  const update = (definition: FlowVariableDefinition): void =>
    save(variables.map((variable: FlowVariableDefinition, i: number) => (i === index ? definition : variable)));

  return (
    <Stack gap={2} data-testid="variables-properties">
      <FormLabel>{t('flows:core.variables.properties.title', 'Variables')}</FormLabel>
      {variables.length === 0 ? (
        <Typography variant="body2" color="text.secondary" data-testid="variables-empty">
          {t('flows:core.variables.properties.empty', 'This node declares no variables yet. Add one to start.')}
        </Typography>
      ) : (
        <Stack gap={1} data-testid="variables-list">
          {variables.map((variable: FlowVariableDefinition, position: number) => {
            const isOpen: boolean = position === index;
            return (
              <Box
                // eslint-disable-next-line react/no-array-index-key -- identifiers can be edited into duplicates while typing
                key={position}
                data-testid={`variables-item-${position}`}
                sx={{
                  border: 1,
                  borderColor: isOpen ? 'primary.main' : 'divider',
                  borderRadius: 1,
                  overflow: 'hidden',
                }}
              >
                <Stack
                  direction="row"
                  alignItems="center"
                  gap={1}
                  role="button"
                  tabIndex={0}
                  aria-expanded={isOpen}
                  onClick={() => setSelected(isOpen ? null : position)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      setSelected(isOpen ? null : position);
                    }
                  }}
                  sx={{
                    px: 1.5,
                    py: 1,
                    cursor: 'pointer',
                    bgcolor: isOpen ? 'action.selected' : 'transparent',
                    '&:hover': {bgcolor: 'action.hover'},
                  }}
                >
                  {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                  <Box flex={1} minWidth={0}>
                    <Typography variant="body2" noWrap>
                      {variable.label ?? variable.identifier}
                    </Typography>
                    <Typography variant="caption" color="text.secondary" noWrap display="block">
                      {variable.identifier}
                    </Typography>
                  </Box>
                  <Tooltip title={t('flows:core.variables.properties.remove', 'Remove')}>
                    <IconButton
                      size="small"
                      color="error"
                      aria-label={t('flows:core.variables.properties.remove', 'Remove')}
                      onClick={(event) => {
                        event.stopPropagation();
                        remove(position);
                      }}
                    >
                      <TrashIcon size={16} />
                    </IconButton>
                  </Tooltip>
                </Stack>
                {isOpen && (
                  <Box sx={{p: 1.5, borderTop: 1, borderColor: 'divider'}}>
                    <VariableEditor
                      connectionOptions={getFlowVariableOptions(variable, connections)}
                      definitions={all}
                      index={offset + position}
                      onChange={update}
                    />
                  </Box>
                )}
              </Box>
            );
          })}
        </Stack>
      )}
      <Stack gap={1}>
        <Button fullWidth variant="outlined" startIcon={<Plus size={16} />} onClick={add} data-testid="variables-add">
          {t('flows:core.variables.properties.add', 'Add')}
        </Button>
        <Button
          fullWidth
          variant="contained"
          startIcon={<Eye size={16} />}
          onClick={() => setIsPreviewOpen(true)}
          data-testid="variables-preview"
        >
          {t('flows:core.variables.properties.preview', 'Preview')}
        </Button>
      </Stack>
      <VariablesPreviewDialog open={isPreviewOpen} variables={variables} onClose={() => setIsPreviewOpen(false)} />
    </Stack>
  );
}

export default VariablesProperties;
