// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, IconButton, Stack, TextField, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import {Fragment, memo, useEffect, useMemo, useReducer, useState, type ReactNode} from 'react';

let nextId = 0;
const generateId = (): string => {
  nextId += 1;
  return `kv-${nextId}`;
};

/**
 * Reducer that maintains a stable list of IDs matched to entries length.
 * Called via dispatch(entries.length) whenever entries change.
 */
const idsReducer = (prev: string[], requiredLength: number): string[] => {
  if (prev.length === requiredLength) {
    return prev;
  }

  if (requiredLength > prev.length) {
    const newIds = Array.from({length: requiredLength - prev.length}, () => generateId());
    return [...prev, ...newIds];
  }

  return prev.slice(0, requiredLength);
};

interface KeyValueRowProps {
  entryKey: string;
  entryValue: string;
  index: number;
  onKeyCommit: (index: number, newKey: string) => void;
  onValueCommit: (index: number, newValue: string) => void;
  onRemove: (index: number) => void;
  keyPlaceholder: string;
  valuePlaceholder: string;
  keyLabel: string;
  valueLabel: string;
  removeLabel: string;
}

/**
 * A single key-value row that manages its own local state.
 * Commits to the parent only on blur to avoid input clobbering during fast typing.
 */
const KeyValueRow = memo(function KeyValueRow({
  entryKey,
  entryValue,
  index,
  onKeyCommit,
  onValueCommit,
  onRemove,
  keyPlaceholder,
  valuePlaceholder,
  keyLabel,
  valueLabel,
  removeLabel,
}: KeyValueRowProps): ReactNode {
  const [localKey, setLocalKey] = useState(entryKey);
  const [localValue, setLocalValue] = useState(entryValue);

  useEffect(() => {
    setLocalKey(entryKey);
  }, [entryKey]);

  useEffect(() => {
    setLocalValue(entryValue);
  }, [entryValue]);

  return (
    <Fragment>
      <TextField
        value={localKey}
        onChange={(e) => setLocalKey(e.target.value)}
        onBlur={() => {
          if (localKey !== entryKey) {
            onKeyCommit(index, localKey);
          }
        }}
        placeholder={keyPlaceholder}
        size="small"
        slotProps={{htmlInput: {'aria-label': keyLabel}}}
      />
      <TextField
        value={localValue}
        onChange={(e) => setLocalValue(e.target.value)}
        onBlur={() => {
          if (localValue !== entryValue) {
            onValueCommit(index, localValue);
          }
        }}
        placeholder={valuePlaceholder}
        size="small"
        slotProps={{htmlInput: {'aria-label': valueLabel}}}
      />
      <Tooltip title={removeLabel}>
        <IconButton size="small" color="error" onClick={() => onRemove(index)} aria-label={removeLabel}>
          <Trash size={18} />
        </IconButton>
      </Tooltip>
    </Fragment>
  );
});

interface KeyValueEditorProps {
  entries: [string, string][];
  onAdd: () => void;
  onRemove: (index: number) => void;
  onKeyChange: (index: number, newKey: string) => void;
  onValueChange: (index: number, newValue: string) => void;
  keyPlaceholder: string;
  valuePlaceholder: string;
  /** Column heading, and accessible name, of every row's key field (e.g. "Label"). */
  keyLabel: string;
  /** Column heading, and accessible name, of every row's value field (e.g. "Attribute"). */
  valueLabel: string;
  /** Text of the button that appends a row, naming what is being added (e.g. "Add Header"). */
  addLabel: string;
  /** Accessible name and tooltip of each row's remove button. */
  removeLabel: string;
}

function KeyValueEditor({
  entries,
  onAdd,
  onRemove,
  onKeyChange,
  onValueChange,
  keyPlaceholder,
  valuePlaceholder,
  keyLabel,
  valueLabel,
  addLabel,
  removeLabel,
}: KeyValueEditorProps): ReactNode {
  // Stable IDs for each entry — used as React keys so rows survive re-renders.
  // useReducer allows synchronous state transitions during render without cascading effects.
  const [ids, dispatchIds] = useReducer(idsReducer, entries.length, (len) =>
    Array.from({length: len}, () => generateId()),
  );

  // Sync IDs with entries length — useReducer dispatch is safe in useMemo
  const syncedIds = useMemo(() => {
    if (ids.length !== entries.length) {
      dispatchIds(entries.length);
    }
    return ids.length === entries.length ? ids : idsReducer(ids, entries.length);
  }, [ids, entries.length]);

  const handleRemove = (index: number): void => {
    // Dispatch a remove: shrink IDs by filtering out the removed index
    // We need to do this before onRemove so the IDs array stays aligned
    dispatchIds(entries.length - 1);
    onRemove(index);
  };

  return (
    <Stack gap={1}>
      {/* One grid for the headings and every row, so each caption sits over the field it names. */}
      <Box
        sx={{
          alignItems: 'center',
          columnGap: 1,
          display: 'grid',
          gridTemplateColumns: '1fr 1fr auto',
          rowGap: 1,
        }}
      >
        {/* Headed once rather than per row: the fields repeat, so labelling each would too. */}
        {entries.length > 0 && (
          <Fragment>
            <Typography variant="caption" color="text.secondary">
              {keyLabel}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {valueLabel}
            </Typography>
            <Box />
          </Fragment>
        )}
        {entries.map(([key, value], index) => (
          <KeyValueRow
            key={syncedIds[index]}
            index={index}
            entryKey={key}
            entryValue={value}
            onKeyCommit={onKeyChange}
            onValueCommit={onValueChange}
            onRemove={handleRemove}
            keyPlaceholder={keyPlaceholder}
            valuePlaceholder={valuePlaceholder}
            keyLabel={keyLabel}
            valueLabel={valueLabel}
            removeLabel={removeLabel}
          />
        ))}
      </Box>
      <Box>
        <Button variant="text" color="primary" size="small" startIcon={<Plus />} onClick={onAdd}>
          {addLabel}
        </Button>
      </Box>
    </Stack>
  );
}

export default KeyValueEditor;
