// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getFlowVariableOptions} from '@thunderid/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import {Box, Button, ButtonBase, FormLabel, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronRight, Eye, Plus, TrashIcon} from '@wso2/oxygen-ui-icons-react';
import {useContext, useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import FlowVariablesContext from '../../../context/FlowVariablesContext';
import ConnectionsLoadError from '../../variables-dialog/ConnectionsLoadError';
import useFlowVariableConnections from '../../variables-dialog/useFlowVariableConnections';
import VariableChangeDialog from '../../variables-dialog/VariableChangeDialog';
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
  const {countBindings, groups, isShared, ownerOuId, removeBindings, renameBindings} = useContext(FlowVariablesContext);
  const {connections, error: connectionsError, refetch: refetchConnections} = useFlowVariableConnections(ownerOuId);
  const [selected, setSelected] = useState<number | null>(null);
  const [isPreviewOpen, setIsPreviewOpen] = useState<boolean>(false);
  const [pending, setPending] = useState<{identifier: string; position: number; type: 'remove' | 'rename'} | null>(
    null,
  );

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

  // Like the other panels, an edit goes through the panel's own change handler, which tells the plugins and updates the
  // node. The flow's variables are only read from the context, for the identifiers of the other variables nodes.
  const save = (next: FlowVariableDefinition[]): void => onChange('data.inputs', next, resource);

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

  const applyRename = (position: number, identifier: string): void => {
    const from: string = variables[position].identifier;

    renameBindings(from, identifier);
    save(
      variables.map((variable: FlowVariableDefinition, i: number) =>
        i === position ? {...variable, identifier} : variable,
      ),
    );
  };

  const applyRemove = (position: number): void => {
    removeBindings(variables[position].identifier);
    remove(position);
  };

  // Steps follow a rename without asking. Only a variable the sharing policies name is confirmed first, as saving the
  // flow without its old identifier deletes the rule and the values units saved for it.
  const requestRename = (position: number, identifier: string): void => {
    if (isShared(variables[position].identifier)) {
      setPending({identifier, position, type: 'rename'});
    } else {
      applyRename(position, identifier);
    }
  };

  const requestRemove = (position: number): void => {
    const {identifier} = variables[position];

    if (isShared(identifier) || countBindings(identifier) > 0) {
      setPending({identifier, position, type: 'remove'});
    } else {
      remove(position);
    }
  };

  const confirmPending = (): void => {
    if (pending?.type === 'rename') applyRename(pending.position, pending.identifier);
    if (pending?.type === 'remove') applyRemove(pending.position);
    setPending(null);
  };

  const pendingDetails = (): string[] => {
    if (!pending) return [];

    const current: string = variables[pending.position]?.identifier ?? '';
    const used: number = countBindings(current);
    const details: string[] = [];

    if (used > 0) {
      details.push(
        pending.type === 'remove'
          ? t('flows:core.variables.change.removeUsed', {
              count: used,
              defaultValue_one: '{{count}} step uses this variable and will be unbound.',
              defaultValue_other: '{{count}} steps use this variable and will be unbound.',
            })
          : t('flows:core.variables.change.renameUsed', {
              count: used,
              defaultValue_one: '{{count}} step uses this variable and will follow the new identifier.',
              defaultValue_other: '{{count}} steps use this variable and will follow the new identifier.',
            }),
      );
    }
    if (isShared(current)) {
      details.push(
        t(
          'flows:core.variables.change.shared',
          'Sharing terms and values saved by organization units for this variable are deleted when you save.',
        ),
      );
    }

    return details;
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
                  sx={{
                    bgcolor: isOpen ? 'action.selected' : 'transparent',
                    '&:hover': {bgcolor: 'action.hover'},
                  }}
                >
                  <ButtonBase
                    aria-expanded={isOpen}
                    onClick={() => setSelected(isOpen ? null : position)}
                    sx={{
                      alignItems: 'center',
                      display: 'flex',
                      flex: 1,
                      gap: 1,
                      justifyContent: 'flex-start',
                      minWidth: 0,
                      px: 1.5,
                      py: 1,
                      textAlign: 'left',
                    }}
                  >
                    {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                    <Box flex={1} minWidth={0}>
                      <Typography variant="body2" component="span" noWrap display="block">
                        {variable.label ?? variable.identifier}
                      </Typography>
                      <Typography variant="caption" component="span" color="text.secondary" noWrap display="block">
                        {variable.identifier}
                      </Typography>
                    </Box>
                  </ButtonBase>
                  <Tooltip title={t('common:actions.remove', 'Remove')}>
                    <IconButton
                      size="small"
                      color="error"
                      aria-label={t('common:actions.remove', 'Remove')}
                      onClick={() => requestRemove(position)}
                      sx={{mr: 1}}
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
                      onRename={(identifier: string) => requestRename(position, identifier)}
                    />
                  </Box>
                )}
              </Box>
            );
          })}
        </Stack>
      )}
      {connectionsError && <ConnectionsLoadError error={connectionsError} onRetry={refetchConnections} />}
      <Stack gap={1}>
        <Button fullWidth variant="outlined" startIcon={<Plus size={16} />} onClick={add} data-testid="variables-add">
          {t('common:actions.add', 'Add')}
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
      <VariableChangeDialog
        open={pending !== null}
        title={
          pending?.type === 'rename'
            ? t('flows:core.variables.change.renameTitle', 'Rename to {{identifier}}?', {
                identifier: pending.identifier,
              })
            : t('flows:core.variables.change.removeTitle', 'Remove {{identifier}}?', {
                identifier: pending?.identifier ?? '',
              })
        }
        details={pendingDetails()}
        confirmLabel={
          pending?.type === 'rename'
            ? t('flows:core.variables.change.renameConfirm', 'Rename')
            : t('common:actions.remove', 'Remove')
        }
        onClose={() => setPending(null)}
        onConfirm={confirmPending}
      />
      <VariablesPreviewDialog
        open={isPreviewOpen}
        ownerOuId={ownerOuId}
        variables={variables}
        onClose={() => setIsPreviewOpen(false)}
      />
    </Stack>
  );
}

export default VariablesProperties;
