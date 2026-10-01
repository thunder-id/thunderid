// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {IconButton, Stack, TextField} from '@wso2/oxygen-ui';
import {memo, useEffect, useMemo, useReducer, useState, type ReactNode} from 'react';

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
    <Stack direction="row" gap={1} alignItems="center">
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
        sx={{flex: 1}}
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
        sx={{flex: 1}}
      />
      <IconButton size="small" onClick={() => onRemove(index)} aria-label="Remove entry">
        &times;
      </IconButton>
    </Stack>
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
}

function KeyValueEditor({
  entries,
  onAdd,
  onRemove,
  onKeyChange,
  onValueChange,
  keyPlaceholder,
  valuePlaceholder,
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
        />
      ))}
      <IconButton size="small" onClick={onAdd} sx={{alignSelf: 'flex-start'}} aria-label="Add entry">
        +
      </IconButton>
    </Stack>
  );
}

export default KeyValueEditor;
