// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdapterDateFns, DatePickers} from '@wso2/oxygen-ui';
import {type JSX} from 'react';

export interface ValidUntilPickerProps {
  id: string;
  /** The RFC 3339 instant the lifespan ends, or empty for one that does not end. */
  value?: string | null;
  /** Called with the chosen instant in RFC 3339, or null once the field is cleared. A partly typed value is not passed on. */
  onChange: (value: string | null) => void;
  /** Called with whether the field holds a value it cannot accept: one that does not parse or, with `disablePast`, a past one. */
  onErrorChange?: (hasError: boolean) => void;
  /** Refuses a moment that has passed. */
  disablePast?: boolean;
  disabled?: boolean;
  helperText?: string;
}

/**
 * A date and time field for when an agent's lifespan ends, shown in the viewer's time zone and sent in UTC.
 * Clearing it removes the end.
 */
export default function ValidUntilPicker({
  id,
  value = undefined,
  onChange,
  onErrorChange = undefined,
  disablePast = false,
  disabled = false,
  helperText = undefined,
}: ValidUntilPickerProps): JSX.Element {
  return (
    <DatePickers.LocalizationProvider dateAdapter={AdapterDateFns}>
      <DatePickers.DateTimePicker
        value={value ? new Date(value) : null}
        disablePast={disablePast}
        disabled={disabled}
        onError={(error) => onErrorChange?.(error !== null)}
        onChange={(date: Date | null) => {
          if (date === null) {
            onChange(null);
          } else if (!Number.isNaN(date.getTime())) {
            onChange(date.toISOString());
          }
        }}
        slotProps={{
          field: {clearable: true},
          textField: {id, size: 'small', fullWidth: true, helperText},
        }}
      />
    </DatePickers.LocalizationProvider>
  );
}
