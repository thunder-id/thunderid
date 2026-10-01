// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Eye, EyeOff, Lock, Plus, RotateCcw, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type ReactNode, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {
  parseKeyValuePairs,
  sanitizeKeyValuePart,
  serializeKeyValuePairs,
  type KeyValuePair,
} from '../utils/keyValuePairs';

interface KeyedPair extends KeyValuePair {
  /** Stable React key, so editing one row never remounts the others. */
  key: number;
}

interface KeyValuePairsFieldProps {
  id: string;
  label?: string;
  /** Stored wire value ("Key: value, Other: value"). */
  value: string;
  onChange: (value: string) => void;
  error?: string;
  hint?: ReactNode;
  /** Masks value inputs and adds a per-row visibility toggle. */
  maskValues?: boolean;
  namePlaceholder?: string;
  addLabel: string;
  hasStoredValue?: boolean;
  replacing?: boolean;
  onReplacingChange?: (replacing: boolean) => void;
  replaceLabel?: string;
  storedHint?: string;
}

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
    rows: pairs.map((pair, index) => ({key: fromSeq + index + 1, ...pair})),
    seq: fromSeq + pairs.length,
    synced: raw,
  };
}

/**
 * Row-per-pair editor for a field the API stores as a single "Key: value" string. Rows are local
 * state (a row being typed has no name yet, so it cannot round-trip through the serialized value),
 * synced back out on every edit and re-derived when the value changes from outside, e.g. a reset.
 */
export default function KeyValuePairsField({
  id,
  label = undefined,
  value,
  onChange,
  error = undefined,
  hint = undefined,
  maskValues = false,
  namePlaceholder = undefined,
  addLabel,
  hasStoredValue = false,
  replacing = false,
  onReplacingChange = undefined,
  replaceLabel = undefined,
  storedHint = undefined,
}: KeyValuePairsFieldProps): JSX.Element {
  const {t} = useTranslation('connections');

  const [state, setState] = useState<RowsState>(() => buildRows(value, 0));
  const [visibleValueKeys, setVisibleValueKeys] = useState<Set<number>>(() => new Set());

  // Re-derive when the value changes from outside the editor, e.g. the detail page's Reset. Rows
  // cannot simply be computed from the value on every render: a row being typed has no name yet,
  // so it would not survive the round trip through the serialized form.
  if (value !== state.synced) {
    setState(buildRows(value, state.seq));
  }

  const {rows} = state;

  const commit = (next: KeyedPair[], seq: number): void => {
    const serialized: string = serializeKeyValuePairs(next);
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

  // Adding a blank row does not change the serialized value, so the parent is not notified.
  const addRow = (): void =>
    setState((prev) => ({...prev, rows: [...prev.rows, {key: prev.seq + 1, name: '', value: ''}], seq: prev.seq + 1}));

  const toggleValueVisibility = (key: number): void => {
    setVisibleValueKeys((previous) => {
      const next = new Set(previous);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const lastRowIsEmpty: boolean =
    rows.length > 0 && rows[rows.length - 1].name.trim() === '' && rows[rows.length - 1].value.trim() === '';

  if (hasStoredValue && !replacing && onReplacingChange && replaceLabel) {
    return (
      <Box>
        <Box sx={{display: 'grid', gridTemplateColumns: '1fr 1fr auto', gap: 1.5, alignItems: 'flex-start'}}>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('form.keyValue.name')}
          </Typography>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('form.keyValue.value')}
          </Typography>
          <Box />
          <TextField
            fullWidth
            disabled
            value="••••••••••••••••"
            slotProps={{
              input: {
                'aria-label': t('form.headers.configuredName', 'Configured header name'),
                startAdornment: (
                  <InputAdornment position="start">
                    <Lock size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
          <TextField
            fullWidth
            disabled
            value="••••••••••••••••"
            slotProps={{
              input: {
                'aria-label': t('form.headers.configuredValue', 'Configured header value'),
                startAdornment: (
                  <InputAdornment position="start">
                    <Lock size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
          <Button
            variant="outlined"
            startIcon={<RotateCcw size={16} />}
            onClick={() => onReplacingChange(true)}
            data-testid={`${id}-replace`}
          >
            {replaceLabel}
          </Button>
        </Box>
        {storedHint && (
          <Typography variant="caption" color="text.secondary">
            {storedHint}
          </Typography>
        )}
      </Box>
    );
  }

  return (
    <FormControl fullWidth error={Boolean(error)}>
      {label && <FormLabel htmlFor={`${id}-name-1`}>{label}</FormLabel>}
      <Stack direction="column" spacing={1.5} sx={label ? {mt: 1} : undefined} data-testid={`${id}-rows`}>
        <Stack direction="row" spacing={1.5}>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('form.keyValue.name')}
          </Typography>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('form.keyValue.value')}
          </Typography>
          <Box sx={{width: 40}} />
        </Stack>

        {rows.map((row, index) => {
          const isOnlyEmptyRow: boolean = rows.length === 1 && row.name === '' && row.value === '';
          const valueVisible: boolean = visibleValueKeys.has(row.key);
          return (
            <Stack key={row.key} direction="row" spacing={1.5} alignItems="center">
              <TextField
                fullWidth
                id={`${id}-name-${index + 1}`}
                value={row.name}
                placeholder={namePlaceholder}
                onChange={(e) => updateRow(row.key, 'name', e.target.value)}
                error={Boolean(error)}
                slotProps={{input: {'aria-label': t('form.keyValue.name')}}}
              />
              <TextField
                fullWidth
                id={`${id}-value-${index + 1}`}
                type={maskValues && !valueVisible ? 'password' : 'text'}
                value={row.value}
                onChange={(e) => updateRow(row.key, 'value', e.target.value)}
                error={Boolean(error)}
                slotProps={{
                  input: {
                    'aria-label': t('form.keyValue.value'),
                    endAdornment: maskValues ? (
                      <InputAdornment position="end">
                        <IconButton
                          onClick={() => toggleValueVisibility(row.key)}
                          edge="end"
                          size="small"
                          aria-label={
                            valueVisible
                              ? t('form.keyValue.hideValue', 'Hide value')
                              : t('form.keyValue.showValue', 'Show value')
                          }
                        >
                          {valueVisible ? <EyeOff size={16} /> : <Eye size={16} />}
                        </IconButton>
                      </InputAdornment>
                    ) : undefined,
                  },
                }}
              />
              {isOnlyEmptyRow ? (
                <Box sx={{width: 40}} />
              ) : (
                <IconButton
                  onClick={() => removeRow(row.key)}
                  aria-label={t('form.keyValue.remove')}
                  data-testid={`${id}-remove-${index + 1}`}
                >
                  <Trash2 size={16} />
                </IconButton>
              )}
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

        {(error ?? hint) && <FormHelperText>{error ?? hint}</FormHelperText>}
      </Stack>
    </FormControl>
  );
}
