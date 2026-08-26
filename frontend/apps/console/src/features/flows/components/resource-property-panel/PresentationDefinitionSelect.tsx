// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetVerifiablePresentations} from '@thunderid/configure-verifiable-credentials';
import {FormControl, FormHelperText, FormLabel, MenuItem, TextField} from '@wso2/oxygen-ui';
import type {ChangeEvent, ReactElement} from 'react';
import {useTranslation} from 'react-i18next';

export interface PresentationDefinitionSelectProps {
  propertyKey: string;
  value: string;
  onChange: (value: string) => void;
  /**
   * Validation message for the bound property, or an empty string when it is valid.
   * @defaultValue ''
   */
  errorMessage?: string;
}

/**
 * A dropdown that lets a flow step pick a configured OpenID4VP presentation
 * definition (by handle) instead of typing a free-text id.
 */
export default function PresentationDefinitionSelect({
  propertyKey,
  value,
  onChange,
  errorMessage = '',
}: PresentationDefinitionSelectProps): ReactElement {
  const {t} = useTranslation();
  const {data, isLoading} = useGetVerifiablePresentations();
  const options = data ?? [];
  const hasError = !!errorMessage;

  return (
    <FormControl fullWidth sx={{mb: 3}} error={hasError}>
      <FormLabel htmlFor={propertyKey}>{t('verifiable-presentations:select.label')}</FormLabel>
      <TextField
        select
        fullWidth
        id={propertyKey}
        value={value ?? ''}
        disabled={isLoading}
        error={hasError}
        onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        placeholder={t('verifiable-presentations:select.placeholder')}
      >
        {options.map((vp) => (
          <MenuItem key={vp.id} value={vp.handle}>
            {vp.name ?? vp.handle}
          </MenuItem>
        ))}
      </TextField>
      {hasError && <FormHelperText error>{errorMessage}</FormHelperText>}
    </FormControl>
  );
}
