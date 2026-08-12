// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowFieldProps} from '../../../models/flow';

type TextInputVariant = 'TEXT_INPUT' | 'EMAIL_INPUT' | 'PHONE_INPUT' | 'NUMBER_INPUT';

const HTML_INPUT_TYPE: Record<TextInputVariant, string> = {
  TEXT_INPUT: 'text',
  EMAIL_INPUT: 'email',
  PHONE_INPUT: 'tel',
  NUMBER_INPUT: 'number',
};

const AUTO_COMPLETE_MAP: Record<TextInputVariant, (ref: string) => string> = {
  TEXT_INPUT: (ref) => {
    if (ref === 'username') return 'username';
    if (ref === 'email') return 'email';
    return 'off';
  },
  EMAIL_INPUT: () => 'email',
  PHONE_INPUT: () => 'tel',
  NUMBER_INPUT: () => 'off',
};

function resolveTextVariant(type: string): TextInputVariant {
  if (type === 'EMAIL_INPUT') return 'EMAIL_INPUT';
  if (type === 'PHONE_INPUT') return 'PHONE_INPUT';
  if (type === 'NUMBER_INPUT') return 'NUMBER_INPUT';
  return 'TEXT_INPUT';
}

export default function TextInputAdapter({
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

  if (!ref || typeof ref !== 'string') return null;

  const variant = resolveTextVariant(String(component.type));
  const htmlType = HTML_INPUT_TYPE[variant];
  const autoComplete = AUTO_COMPLETE_MAP[variant](ref);
  const autoFocus = ref === 'username';
  const hasError = !!(touched?.[ref] && fieldErrors?.[ref]);
  const value = values[ref] ?? '';

  return (
    <FormControl required={component.required} className={cn('Flow--textInput', 'FormControl--root')}>
      <FormLabel htmlFor={ref} className={cn('Label--root')}>
        {t(resolve(component.label)!)}
      </FormLabel>
      <TextField
        fullWidth
        className={cn('TextField--root')}
        id={ref}
        name={ref}
        type={htmlType}
        placeholder={t(resolve(component.placeholder) ?? component.placeholder ?? '')}
        autoComplete={autoComplete}
        // eslint-disable-next-line jsx-a11y/no-autofocus
        autoFocus={autoFocus}
        required={component.required}
        variant="outlined"
        disabled={isLoading}
        error={hasError}
        helperText={hasError ? fieldErrors?.[ref] : undefined}
        color={hasError ? 'error' : 'primary'}
        value={value}
        onChange={(e) => onInputChange(ref, e.target.value)}
        onBlur={() => onBlur?.(ref)}
      />
    </FormControl>
  );
}
