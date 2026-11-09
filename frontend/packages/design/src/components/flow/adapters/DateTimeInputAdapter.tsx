// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import type {FlowFieldProps} from '../../../models/flow';
import DateTimePickerField from '../../DateTimePickerField';

export default function DateTimeInputAdapter({
  component,
  values,
  touched,
  fieldErrors,
  isLoading,
  resolve,
  onInputChange,
  onBlur,
}: FlowFieldProps): JSX.Element | null {
  const {ref} = component;

  if (!ref || typeof ref !== 'string') return null;

  const hasError = !!(touched?.[ref] && fieldErrors?.[ref]);
  const hint = component.hint ? (resolve(component.hint) ?? component.hint) : undefined;

  return (
    <DateTimePickerField
      id={ref}
      value={values[ref] ?? null}
      label={resolve(component.label)}
      required={component.required}
      disabled={isLoading}
      helperText={hasError ? fieldErrors?.[ref] : hint}
      onChange={(value) => onInputChange(ref, value ?? '')}
      onBlur={() => onBlur?.(ref)}
    />
  );
}
