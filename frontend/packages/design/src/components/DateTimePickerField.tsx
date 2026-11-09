// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdapterDateFns, DatePickers} from '@wso2/oxygen-ui';
import {type JSX, type ReactNode} from 'react';

export interface DateTimePickerFieldProps {
  id: string;
  value?: string | null;
  onChange: (value: string | null) => void;
  onBlur?: () => void;
  onErrorChange?: (hasError: boolean) => void;
  disablePast?: boolean;
  disabled?: boolean;
  label?: ReactNode;
  required?: boolean;
  helperText?: string;
}

export default function DateTimePickerField({
  id,
  value = undefined,
  onChange,
  onBlur = undefined,
  onErrorChange = undefined,
  disablePast = false,
  disabled = false,
  label = undefined,
  required = false,
  helperText = undefined,
}: DateTimePickerFieldProps): JSX.Element {
  return (
    <DatePickers.LocalizationProvider dateAdapter={AdapterDateFns}>
      <DatePickers.DateTimePicker
        value={value ? new Date(value) : null}
        disablePast={disablePast}
        disabled={disabled}
        onError={(error) => onErrorChange?.(error !== null)}
        onChange={(date: Date | null) => {
          if (date === null) onChange(null);
          else if (!Number.isNaN(date.getTime())) onChange(date.toISOString());
        }}
        slotProps={{
          field: {clearable: true},
          textField: {id, size: 'small', fullWidth: true, label, required, helperText, onBlur},
        }}
      />
    </DatePickers.LocalizationProvider>
  );
}
