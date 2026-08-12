// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, InputAdornment, MenuItem, Select, Stack, TextField, Tooltip} from '@wso2/oxygen-ui';
import {Lock, Trash} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {AllowedOriginTypes, type AllowedOriginType} from '../../models/allowedOriginRow';

/** Keeps the lock on a read-only row aligned with the remove button on an editable one. */
const ROW_ACTION_WIDTH = 40;

const TYPE_SELECT_WIDTH = 116;

/**
 * Props for {@link AllowedOriginRow}.
 *
 * @public
 */
export interface AllowedOriginRowProps {
  /** Whether the row holds a literal origin or a regex pattern. */
  type: AllowedOriginType;

  /** The origin or pattern text, without the decorative delimiters. */
  value: string;

  /** Already-translated blocking message. */
  error?: string;

  /** Already-translated non-blocking caution, shown only when there is no error. */
  warning?: string;

  /** Whether the entry is managed declaratively and cannot be edited or removed. */
  locked?: boolean;

  /** Already-translated placeholder for a literal origin. */
  originPlaceholder: string;

  /** Already-translated placeholder for a regex pattern. */
  regexPlaceholder: string;

  /** Already-translated accessible name for the type selector. */
  typeLabel: string;

  /** Already-translated label for the literal origin option. */
  originOptionLabel: string;

  /** Already-translated label for the regex option. */
  regexOptionLabel: string;

  /** Already-translated accessible name for the remove action. */
  removeLabel: string;

  /** Already-translated tooltip explaining why a locked row cannot be edited. */
  lockedLabel?: string;

  /** Test id applied to the row, so a row's controls can be located together. */
  testId?: string;

  onTypeChange?: (type: AllowedOriginType) => void;
  onChange?: (value: string) => void;
  onBlur?: () => void;
  onRemove?: () => void;
}

/**
 * A single allowed-origin row: an explicit Origin/Regex selector, the value field, and a remove
 * action. Regex rows render monospaced between `/` delimiters so a pattern is never mistaken for a
 * URL. The delimiters are decoration and are not part of the value.
 *
 * Every string is injected already translated, so the Settings page and the application creation
 * wizard can render the same row from their own i18n namespaces.
 *
 * @public
 */
export default function AllowedOriginRow({
  type,
  value,
  error = undefined,
  warning = undefined,
  locked = false,
  originPlaceholder,
  regexPlaceholder,
  typeLabel,
  originOptionLabel,
  regexOptionLabel,
  removeLabel,
  lockedLabel = undefined,
  testId = undefined,
  onTypeChange = undefined,
  onChange = undefined,
  onBlur = undefined,
  onRemove = undefined,
}: AllowedOriginRowProps): JSX.Element {
  const isRegex = type === AllowedOriginTypes.REGEX;
  const helperText = error ?? warning;

  /**
   * One of the `/` delimiters that frame a regex row. They are hidden from assistive technology and
   * live outside the input value, so the field still reports the raw pattern (which contains slashes
   * of its own).
   *
   * @param position - Which side of the field to render
   * @returns The decorative adornment
   */
  const delimiter = (position: 'start' | 'end'): JSX.Element => (
    <InputAdornment position={position} aria-hidden sx={{color: 'text.disabled'}}>
      /
    </InputAdornment>
  );

  return (
    <Stack direction="row" spacing={1} alignItems="flex-start" data-componentid={testId}>
      <Select
        size="small"
        value={type}
        disabled={locked}
        onChange={(event) => onTypeChange?.(event.target.value as AllowedOriginType)}
        inputProps={{'aria-label': typeLabel}}
        sx={{width: TYPE_SELECT_WIDTH, flex: 'none'}}
      >
        <MenuItem value={AllowedOriginTypes.ORIGIN}>{originOptionLabel}</MenuItem>
        <MenuItem value={AllowedOriginTypes.REGEX}>{regexOptionLabel}</MenuItem>
      </Select>

      <TextField
        fullWidth
        size="small"
        value={value}
        // A locked row always has a value, so a placeholder would only ever be dead markup that
        // makes a declarative entry look like an empty editable one.
        placeholder={locked ? undefined : isRegex ? regexPlaceholder : originPlaceholder}
        error={Boolean(error)}
        helperText={helperText}
        onChange={(event) => onChange?.(event.target.value)}
        onBlur={onBlur}
        slotProps={{
          input: {
            readOnly: locked,
            startAdornment: isRegex ? delimiter('start') : undefined,
            endAdornment: isRegex ? delimiter('end') : undefined,
            sx: isRegex ? {fontFamily: 'monospace'} : undefined,
          },
        }}
        sx={{
          flex: 1,
          ...(locked ? {opacity: 0.65} : {}),
          ...(!error && warning ? {'& .MuiFormHelperText-root': {color: 'warning.main'}} : {}),
        }}
      />

      {locked ? (
        <Tooltip title={lockedLabel ?? ''}>
          <Box
            sx={{
              width: ROW_ACTION_WIDTH,
              flex: 'none',
              display: 'inline-flex',
              justifyContent: 'center',
              mt: 1,
              color: 'text.disabled',
            }}
          >
            <Lock size={18} />
          </Box>
        </Tooltip>
      ) : (
        <Tooltip title={removeLabel}>
          <IconButton
            aria-label={removeLabel}
            color="error"
            onClick={onRemove}
            sx={{width: ROW_ACTION_WIDTH, flex: 'none', mt: 0.5}}
          >
            <Trash size={18} />
          </IconButton>
        </Tooltip>
      )}
    </Stack>
  );
}
