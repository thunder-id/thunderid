// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Box,
  Chip,
  Collapse,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronRight, Trash2} from '@wso2/oxygen-ui-icons-react';
import {Fragment, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {SetValueButton, ValueName, ValueText} from './ValueCells';
import ValueEditDialog from './ValueEditDialog';
import {pendingWriteOf, useDeleteStoredValue, usePendingValueWrites} from '../api/useStoredValues';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import useGatewayValueAccess from '../hooks/useGatewayValueAccess';
import type {StoredValue, ValueKind, ValueReference} from '../models/deploy';
import groupByResource, {humanize, type ResourceGroup, type TypeGroup} from '../utils/groupByResource';
import {isHeld} from '../utils/valueRows';

/** One name a gateway holds or is asked to hold, with what refers to it. */
export interface ValueRow {
  name: string;
  /** What the gateway holds: a variable's value, or a secret's presence. */
  stored?: StoredValue;
  /** The places the running version refers to it. */
  references: ValueReference[];
  /**
   * The places the configuration as it stands refers to it, kept only for a name the running version does
   * not refer to, so that it can be shown with the resource it belongs to.
   */
  current: ValueReference[];
  /** Only resources not shown here use it, so it is listed but neither set nor removed here. */
  hiddenUse?: boolean;
}

export interface ValuesTableProps {
  gatewayId: string;
  kind: ValueKind;
  rows: ValueRow[];
}

// What each type's values are listed under, for the types a configuration commonly holds.
const TYPE_LABELS: Record<string, string> = {
  agent: 'Agents',
  agent_type: 'Agent types',
  application: 'Applications',
  connection: 'Connections',
  flow: 'Flows',
  group: 'Groups',
  layout: 'Layouts',
  organization_unit: 'Organization units',
  resource_server: 'Resource servers',
  role: 'Roles',
  server_config: 'Server configuration',
  theme: 'Themes',
  translation: 'Translations',
  user: 'Users',
  user_type: 'User types',
};

const countOf = (resources: ResourceGroup[]): number =>
  resources.reduce((total: number, resource: ResourceGroup) => total + resource.rows.length, 0);

/**
 * A gateway's variables or secrets grouped by the type of resource that uses them and then by the
 * resource, each type a row that opens to its values, as the import and export views list resources.
 * Values the running version refers to that the gateway does not hold come first, open and marked, so
 * they are set where they are seen. A value is set in the same dialog as on the list of what the gateway
 * holds. A secret's value is never shown; setting one replaces it.
 */
export default function ValuesTable({gatewayId, kind, rows}: ValuesTableProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const {canValue} = useGatewayValueAccess();
  const canSet = canValue(kind, 'update');
  const canDelete = canValue(kind, 'delete');
  const deleteValue = useDeleteStoredValue(gatewayId, kind);
  // Writes started in any view of the gateway, so a value is not set while it is being removed, nor
  // removed while it is being saved: the one that lands last would undo the other.
  const pending = usePendingValueWrites(gatewayId);
  const [editing, setEditing] = useState<ValueRow | null>(null);
  // What was opened or closed by hand. Untouched, the missing values are open and the rest closed.
  const [toggled, setToggled] = useState<Record<string, boolean>>({});
  const secret = kind === 'secret';

  const missing = useMemo(
    () => groupByResource(rows.filter((row: ValueRow) => !isHeld(row, secret) && row.references.length > 0)),
    [rows, secret],
  );
  const groups = useMemo(
    () => groupByResource(rows.filter((row: ValueRow) => isHeld(row, secret) || row.references.length === 0)),
    [rows, secret],
  );

  const typeLabel = (resourceType: string | null): string =>
    resourceType
      ? t(`values.types.${resourceType}`, {defaultValue: TYPE_LABELS[resourceType] ?? humanize(resourceType)})
      : t('values.other', 'Not used by the version this gateway runs');

  if (rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{py: 2}}>
        {secret
          ? t('values.noSecrets', 'This gateway holds no secrets.')
          : t('values.noVariables', 'This gateway holds no variables.')}
      </Typography>
    );
  }

  const actionsCell = (row: ValueRow): JSX.Element | null => {
    if (row.hiddenUse) return null;
    const held = isHeld(row, secret);
    return (
      <>
        {canSet && (
          <SetValueButton
            row={row}
            kind={kind}
            onClick={() => {
              deleteValue.reset();
              setEditing(row);
            }}
            disabled={pending.removing.has(pendingWriteOf(kind, row.name))}
          />
        )}
        {held && canDelete && (
          <Tooltip title={t('values.delete', 'Remove')}>
            <IconButton
              size="small"
              color="error"
              aria-label={t('values.deleteLabel', 'Remove {{name}}', {name: row.name})}
              onClick={() => deleteValue.mutate(row.name)}
              disabled={deleteValue.isPending || pending.setting.has(pendingWriteOf(kind, row.name))}
            >
              <Trash2 size={16} />
            </IconButton>
          </Tooltip>
        )}
      </>
    );
  };

  const valuesTable = (resources: ResourceGroup[]): JSX.Element => (
    <Box sx={{bgcolor: 'background.default', px: 2, py: 1.5}}>
      <Paper variant="outlined" sx={{overflow: 'hidden'}}>
        <Table size="small">
          <TableHead>
            <TableRow sx={{'& th': {bgcolor: 'action.hover'}}}>
              <TableCell>{t('values.name', 'Name')}</TableCell>
              <TableCell>{t('values.value', 'Value')}</TableCell>
              <TableCell align="right" />
            </TableRow>
          </TableHead>
          <TableBody>
            {resources.map((resource: ResourceGroup) => (
              <Fragment key={resource.key}>
                {resource.label && (
                  <TableRow>
                    <TableCell colSpan={3} sx={{py: 0.75}}>
                      <Typography component="h4" variant="subtitle2">
                        {resource.label}
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}
                {resource.rows.map((row: ValueRow) => (
                  <TableRow key={row.name} data-testid={`value-row-${row.name}`}>
                    <TableCell sx={{pl: resource.label ? 3 : 2, verticalAlign: 'top', width: '40%'}}>
                      <ValueName row={row} />
                    </TableCell>
                    <TableCell sx={{verticalAlign: 'top', minWidth: 280}}>
                      <ValueText row={row} kind={kind} />
                    </TableCell>
                    <TableCell align="right" sx={{verticalAlign: 'top', whiteSpace: 'nowrap'}}>
                      {actionsCell(row)}
                    </TableCell>
                  </TableRow>
                ))}
              </Fragment>
            ))}
          </TableBody>
        </Table>
      </Paper>
    </Box>
  );

  const section = (key: string, label: string, resources: ResourceGroup[], alert = false): JSX.Element => {
    const open = toggled[key] ?? alert;
    const toggle = (): void => setToggled((previous) => ({...previous, [key]: !open}));
    return (
      <Fragment key={key}>
        <TableRow
          hover
          sx={{cursor: 'pointer', ...(alert ? {'& td': {color: 'error.main'}} : {})}}
          onClick={toggle}
          data-testid={`values-group-${key}`}
        >
          <TableCell sx={{width: 48, py: 1, pl: 1.5}}>
            <IconButton
              size="small"
              aria-expanded={open}
              aria-label={label}
              color={alert ? 'error' : 'default'}
              onClick={(event) => {
                event.stopPropagation();
                toggle();
              }}
            >
              <Box
                component="span"
                sx={{display: 'flex', transform: open ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s'}}
              >
                <ChevronRight size={16} />
              </Box>
            </IconButton>
          </TableCell>
          <TableCell>
            <Typography component="h3" variant="body2" sx={{fontWeight: alert ? 600 : 500}}>
              {label}
            </Typography>
          </TableCell>
          <TableCell align="right" sx={{width: 120}}>
            {alert ? (
              <Chip size="small" color="error" label={countOf(resources)} />
            ) : (
              <Typography variant="body2" fontWeight={600}>
                {countOf(resources)}
              </Typography>
            )}
          </TableCell>
        </TableRow>
        <TableRow>
          <TableCell colSpan={3} sx={{p: 0, border: 0}}>
            <Collapse in={open} timeout="auto" unmountOnExit>
              {valuesTable(resources)}
            </Collapse>
          </TableCell>
        </TableRow>
      </Fragment>
    );
  };

  return (
    <Stack spacing={2}>
      {deleteValue.error && (
        <Alert severity="error" onClose={() => deleteValue.reset()}>
          {getErrorMessage(
            deleteValue.error,
            tForErrors,
            'deploy:values.deleteFailed',
            'The value could not be removed.',
          )}
        </Alert>
      )}
      <Paper variant="outlined" sx={{overflow: 'hidden', ...(missing.length > 0 ? {borderColor: 'error.main'} : {})}}>
        <Table size="small" data-testid={`values-${kind}`}>
          <TableBody>
            {missing.length > 0 &&
              section(
                'missing',
                t('values.missingTitle', 'Missing values'),
                // The missing values mix types, so each resource is named with its type.
                missing.flatMap((group: TypeGroup) =>
                  group.resources.map((resource: ResourceGroup) => ({
                    ...resource,
                    label: resource.label ? `${typeLabel(group.resourceType)}: ${resource.label}` : resource.label,
                  })),
                ),
                true,
              )}
            {groups.map((group: TypeGroup) =>
              section(group.resourceType ?? 'other', typeLabel(group.resourceType), group.resources),
            )}
          </TableBody>
        </Table>
      </Paper>
      <ValueEditDialog gatewayId={gatewayId} kind={kind} row={editing} onClose={() => setEditing(null)} />
    </Stack>
  );
}
