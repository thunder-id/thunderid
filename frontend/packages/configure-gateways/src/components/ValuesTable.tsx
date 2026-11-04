// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Collapse,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Check, ChevronRight, Pencil, Trash2, X} from '@wso2/oxygen-ui-icons-react';
import {Fragment, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useDeleteStoredValue, useSetStoredValue} from '../api/useStoredValues';
import type {StoredValue, ValueKind, ValueReference} from '../models/deploy';
import groupByResource, {humanize, type ResourceGroup, type TypeGroup} from '../utils/groupByResource';
import {fromListText, toListText} from '../utils/listValue';

/** One name a gateway holds or is asked to hold, with what refers to it. */
export interface ValueRow {
  name: string;
  /** What the gateway holds: a variable's value, or a secret's presence. */
  stored?: StoredValue;
  /** The places the running version refers to it. */
  references: ValueReference[];
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

const isHeld = (row: ValueRow, secret: boolean): boolean =>
  secret ? Boolean(row.stored) && row.stored?.exists !== false : row.stored?.value !== undefined;

const countOf = (resources: ResourceGroup[]): number =>
  resources.reduce((total: number, resource: ResourceGroup) => total + resource.rows.length, 0);

/**
 * A gateway's variables or secrets grouped by the type of resource that uses them and then by the
 * resource, each type a row that opens to its values, as the import and export views list resources.
 * Values the running version refers to that the gateway does not hold come first, open and marked, so
 * they are set where they are seen. A secret's value is never shown; setting one replaces it.
 */
export default function ValuesTable({gatewayId, kind, rows}: ValuesTableProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const setValue = useSetStoredValue(gatewayId, kind);
  const deleteValue = useDeleteStoredValue(gatewayId, kind);
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [error, setError] = useState<string | null>(null);
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
  const fieldLabel = (field: string | undefined): string =>
    field ? t(`values.fields.${field}`, {defaultValue: humanize(field)}) : t('values.value', 'Value');

  const start = (row: ValueRow): void => {
    setError(null);
    setEditing(row.name);
    setDraft(secret ? '' : toListText(row.stored?.value ?? ''));
  };
  const isList = (row: ValueRow): boolean =>
    row.references.some((reference: ValueReference) => reference.list === true);
  const save = async (row: ValueRow): Promise<void> => {
    setError(null);
    try {
      await setValue.mutateAsync({
        name: row.name,
        value: isList(row) ? fromListText(draft) : draft,
        description: row.stored?.description,
      });
      setEditing(null);
    } catch (caught: unknown) {
      setError(
        caught instanceof Error && caught.message
          ? caught.message
          : t('values.saveFailed', 'The value could not be saved.'),
      );
    }
  };
  const remove = async (row: ValueRow): Promise<void> => {
    setError(null);
    try {
      await deleteValue.mutateAsync(row.name);
    } catch (caught: unknown) {
      setError(
        caught instanceof Error && caught.message
          ? caught.message
          : t('values.deleteFailed', 'The value could not be removed.'),
      );
    }
  };

  if (rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{py: 2}}>
        {secret
          ? t('values.noSecrets', 'This gateway holds no secrets.')
          : t('values.noVariables', 'This gateway holds no variables.')}
      </Typography>
    );
  }

  const nameCell = (row: ValueRow): JSX.Element => {
    const others = [
      ...new Set(
        row.references
          .slice(1)
          .map((reference: ValueReference) =>
            [reference.resourceName ?? reference.resourceId, fieldLabel(reference.field)].filter(Boolean).join(' · '),
          ),
      ),
    ];
    return (
      <Box>
        <Stack direction="row" spacing={1} alignItems="center">
          <Typography variant="body2" sx={{fontWeight: 500}}>
            {row.references.length > 0 ? fieldLabel(row.references[0].field) : row.name}
          </Typography>
          {others.length > 0 && (
            <Tooltip title={others.join(', ')}>
              <Typography component="span" variant="body2" color="text.secondary">
                {t('values.more', '+{{count}} more', {count: others.length})}
              </Typography>
            </Tooltip>
          )}
        </Stack>
        {row.references.length > 0 && (
          <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
            {row.name}
          </Typography>
        )}
      </Box>
    );
  };

  const valueCell = (row: ValueRow): JSX.Element => {
    const held = isHeld(row, secret);
    if (editing === row.name) {
      return (
        <TextField
          size="small"
          fullWidth
          type={secret ? 'password' : 'text'}
          autoComplete={secret ? 'new-password' : 'off'}
          multiline={!secret && isList(row)}
          minRows={!secret && isList(row) ? 2 : undefined}
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder={secret ? t('values.secretPlaceholder', 'Enter a new value to replace') : undefined}
          helperText={!secret && isList(row) ? t('values.listHint', 'One per line.') : undefined}
          inputProps={{'aria-label': row.name}}
          slotProps={{input: {sx: {fontFamily: 'monospace', fontSize: 13}}}}
        />
      );
    }
    if (!held) return <Chip size="small" color="error" variant="outlined" label={t('values.notSet', 'Not set')} />;
    if (secret) return <Chip size="small" variant="outlined" label={t('values.stored', 'Stored')} />;
    return (
      <Typography
        variant="body2"
        sx={{fontFamily: 'monospace', fontSize: 13, whiteSpace: 'pre-wrap', wordBreak: 'break-all'}}
      >
        {toListText(row.stored?.value ?? '') || t('values.empty', '(empty)')}
      </Typography>
    );
  };

  const actionsCell = (row: ValueRow): JSX.Element => {
    const held = isHeld(row, secret);
    if (editing === row.name) {
      return (
        <>
          <Tooltip title={t('values.save', 'Save')}>
            <span>
              <IconButton
                size="small"
                aria-label={t('values.save', 'Save')}
                onClick={() => void save(row)}
                disabled={setValue.isPending || (secret && draft === '')}
              >
                {setValue.isPending ? <CircularProgress size={14} /> : <Check size={16} />}
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t('values.cancel', 'Cancel')}>
            <IconButton size="small" aria-label={t('values.cancel', 'Cancel')} onClick={() => setEditing(null)}>
              <X size={16} />
            </IconButton>
          </Tooltip>
        </>
      );
    }
    return (
      <>
        <Button
          size="small"
          variant="text"
          startIcon={<Pencil size={14} />}
          onClick={() => start(row)}
          aria-label={t('values.setLabel', 'Set {{name}}', {name: row.name})}
        >
          {held ? (secret ? t('values.replace', 'Replace') : t('values.edit', 'Edit')) : t('values.set', 'Set')}
        </Button>
        {held && (
          <Tooltip title={t('values.delete', 'Remove')}>
            <IconButton
              size="small"
              color="error"
              aria-label={t('values.deleteLabel', 'Remove {{name}}', {name: row.name})}
              onClick={() => void remove(row)}
              disabled={deleteValue.isPending}
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
                      {nameCell(row)}
                    </TableCell>
                    <TableCell sx={{verticalAlign: 'top', minWidth: 280}}>{valueCell(row)}</TableCell>
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
      {error && <Alert severity="error">{error}</Alert>}
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
    </Stack>
  );
}
