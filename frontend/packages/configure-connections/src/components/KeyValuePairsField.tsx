// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, FormLabel, IconButton, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Plus, RotateCcw, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type ReactNode, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {parseKeyValuePairs, sanitizeKeyValuePart, type KeyValuePair} from '../utils/keyValuePairs';

interface KeyedPair extends KeyValuePair {
  /** Stable React key, so editing one row never remounts the others. */
  key: number;
  /** Mask value returned by the API while a stored credential is being replaced. */
  storedValue?: string;
  /** Original stored header name, restored when replacement is cancelled. */
  storedName?: string;
  /** Whether the stored name and value are being replaced. */
  replacing?: boolean;
}

interface KeyValuePairsFieldProps {
  id: string;
  label: string;
  /** Serialized form value. The parent maps it to the API's structured field. */
  value: string;
  onChange: (value: string) => void;
  hint?: ReactNode;
  namePlaceholder?: string;
  addLabel: string;
  hasStoredValues?: boolean;
  error?: string;
}

const maskedSecretValue = '******';

interface RowsState {
  rows: KeyedPair[];
  /** Highest key handed out so far, carried in state so new rows never reuse one. */
  seq: number;
  /** Wire value these rows were built from, so a change made outside re-derives them. */
  synced: string;
}

/** Rows for the given wire value, always keeping one blank row to type into. */
function buildRows(raw: string, fromSeq: number): RowsState {
  const parsed: KeyValuePair[] = parseKeyValuePairs(raw);
  const pairs: KeyValuePair[] = parsed.length > 0 ? parsed : [{name: '', value: ''}];
  return {
    rows: pairs.map((pair, index) => ({
      key: fromSeq + index + 1,
      name: pair.name,
      value: pair.value,
      storedValue: pair.value === maskedSecretValue ? maskedSecretValue : undefined,
      storedName: pair.value === maskedSecretValue ? pair.name : undefined,
    })),
    seq: fromSeq + pairs.length,
    synced: raw,
  };
}

/**
 * Row-per-pair editor. Rows are local state so a newly added blank row can remain visible without
 * changing the serialized form value. Edits are synced out and external changes, e.g. a reset,
 * re-derive the rows.
 */
export default function KeyValuePairsField({
  id,
  label,
  value,
  onChange,
  hint = undefined,
  namePlaceholder = undefined,
  addLabel,
  hasStoredValues = false,
  error = undefined,
}: KeyValuePairsFieldProps): JSX.Element {
  const {t} = useTranslation('connections');

  const [state, setState] = useState<RowsState>(() => buildRows(value, 0));

  // Re-derive when the value changes from outside the editor, e.g. the detail page's Reset. Rows
  // cannot simply be computed from the value on every render: a row being typed has no name yet,
  // so it would not survive the round trip through the serialized form.
  if (value !== state.synced) {
    setState(buildRows(value, state.seq));
  }

  const {rows} = state;

  const commit = (next: KeyedPair[], seq: number): void => {
    const serialized: string = next
      .filter((row) => row.replacing === true || row.name.trim() !== '' || row.value.trim() !== '')
      .map((row) => {
        if (row.storedValue !== undefined && !row.replacing) {
          return `${row.storedName ?? row.name}: ${row.storedValue}`;
        }
        return `${row.name.trim()}: ${row.value.trim()}`;
      })
      .join(', ');
    setState({rows: next, seq, synced: serialized});
    onChange(serialized);
  };

  const updateRow = (key: number, part: 'name' | 'value', text: string): void => {
    const next: KeyedPair[] = rows.map((row) =>
      row.key === key ? {...row, [part]: sanitizeKeyValuePart(text, part)} : row,
    );
    commit(next, state.seq);
  };

  const removeRow = (key: number): void => {
    const remaining: KeyedPair[] = rows.filter((row) => row.key !== key);
    if (remaining.length > 0) {
      commit(remaining, state.seq);
      return;
    }
    commit([{key: state.seq + 1, name: '', value: ''}], state.seq + 1);
  };

  const replaceRow = (key: number): void => {
    const next: KeyedPair[] = rows.map((row) => (row.key === key ? {...row, replacing: true, value: ''} : row));
    commit(next, state.seq);
  };

  const cancelReplacingRow = (key: number): void => {
    const next: KeyedPair[] = rows.map((row) =>
      row.key === key
        ? {...row, name: row.storedName ?? row.name, value: row.storedValue ?? '', replacing: false}
        : row,
    );
    commit(next, state.seq);
  };

  // Adding a blank row does not change the serialized value, so the parent is not notified.
  const addRow = (): void =>
    setState((prev) => ({...prev, rows: [...prev.rows, {key: prev.seq + 1, name: '', value: ''}], seq: prev.seq + 1}));

  const lastRowIsEmpty: boolean =
    rows.length > 0 && rows[rows.length - 1].name.trim() === '' && rows[rows.length - 1].value.trim() === '';

  return (
    <Box>
      <FormLabel htmlFor={`${id}-name-1`}>{label}</FormLabel>
      <Stack direction="column" spacing={1.5} sx={{mt: 1}} data-testid={`${id}-rows`}>
        <Stack direction="row" spacing={1.5}>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('form.keyValue.name')}
          </Typography>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('form.keyValue.value')}
          </Typography>
          <Box sx={{width: 132}} />
        </Stack>

        {rows.map((row, index) => {
          const isOnlyEmptyRow: boolean = rows.length === 1 && row.name === '' && row.value === '';
          const isStoredRow: boolean = hasStoredValues && row.storedValue !== undefined;
          const isStoredValueLocked: boolean = isStoredRow && !row.replacing;
          return (
            <Stack key={row.key} direction="row" spacing={1.5} alignItems="center">
              <TextField
                fullWidth
                id={`${id}-name-${index + 1}`}
                value={row.name}
                placeholder={namePlaceholder}
                disabled={isStoredValueLocked}
                onChange={(e) => updateRow(row.key, 'name', e.target.value)}
                slotProps={{input: {'aria-label': t('form.keyValue.name')}}}
              />
              <TextField
                fullWidth
                id={`${id}-value-${index + 1}`}
                value={isStoredValueLocked ? '••••••••••••••••' : row.value}
                disabled={isStoredValueLocked}
                onChange={(e) => updateRow(row.key, 'value', e.target.value)}
                slotProps={{input: {'aria-label': t('form.keyValue.value')}}}
              />
              <Stack direction="row" spacing={0.5} sx={{width: 132, justifyContent: 'flex-end'}}>
                {isStoredRow && !row.replacing && (
                  <Button
                    variant="outlined"
                    size="small"
                    startIcon={<RotateCcw size={16} />}
                    onClick={() => replaceRow(row.key)}
                    data-testid={`${id}-update-${index + 1}`}
                  >
                    {t('form.keyValue.update')}
                  </Button>
                )}
                {isStoredRow && row.replacing && (
                  <Button
                    variant="text"
                    size="small"
                    onClick={() => cancelReplacingRow(row.key)}
                    data-testid={`${id}-cancel-${index + 1}`}
                  >
                    {t('form.keyValue.cancel')}
                  </Button>
                )}
                {!isOnlyEmptyRow && (
                  <IconButton
                    onClick={() => removeRow(row.key)}
                    aria-label={t('form.keyValue.remove')}
                    data-testid={`${id}-remove-${index + 1}`}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                )}
              </Stack>
            </Stack>
          );
        })}

        <Box>
          <Button
            variant="text"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={addRow}
            disabled={lastRowIsEmpty}
            data-testid={`${id}-add`}
          >
            {addLabel}
          </Button>
        </Box>

        {(error ?? hint) && (
          <Typography variant="caption" color={error ? 'error' : 'text.secondary'}>
            {error ?? hint}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
