// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Checkbox, FormControl, FormControlLabel, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowFieldProps} from '../../../models/flow';

export default function CheckboxAdapter({
  component,
  values,
  touched,
  fieldErrors,
  isLoading,
  resolve,
  onInputChange,
  onBlur,
}: FlowFieldProps): JSX.Element | null {
  const {t} = useTranslation();
  const {ref} = component;
  const isBound = typeof ref === 'string' && !!ref;
  const currentValue = isBound ? values[ref] : undefined;

  // A boolean field carries a value even when the user never touches it, so seed the unchecked
  // state. Without this the attribute is absent from the submission and a required boolean can
  // never be satisfied.
  useEffect(() => {
    if (isBound && currentValue === undefined) {
      onInputChange(ref, 'false');
    }
  }, [isBound, ref, currentValue, onInputChange]);

  if (!isBound) return null;

  const hasError = !!(touched?.[ref] && fieldErrors?.[ref]);

  return (
    <FormControl required={component.required} className={cn('Flow--checkbox', 'FormControl--root')} error={hasError}>
      <FormControlLabel
        className={cn('FormControlLabel--root')}
        control={
          <Checkbox
            className={cn('Checkbox--root')}
            id={ref}
            name={ref}
            size="small"
            disabled={isLoading}
            checked={currentValue === 'true'}
            onChange={(e) => onInputChange(ref, String(e.target.checked))}
            onBlur={() => onBlur?.(ref)}
          />
        }
        label={t(resolve(component.label)!)}
      />
      {hasError && (
        <Typography variant="caption" color="error.main">
          {fieldErrors?.[ref]}
        </Typography>
      )}
    </FormControl>
  );
}
