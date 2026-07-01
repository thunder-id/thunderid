/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {FormControl, FormLabel, MenuItem, TextField} from '@wso2/oxygen-ui';
import type {ChangeEvent, ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useGetVerifiablePresentations from '@/features/verifiable-presentations/api/useGetVerifiablePresentations';

export interface PresentationDefinitionSelectProps {
  propertyKey: string;
  value: string;
  onChange: (value: string) => void;
}

/**
 * A dropdown that lets a flow step pick a configured OpenID4VP presentation
 * definition (by handle) instead of typing a free-text id.
 */
export default function PresentationDefinitionSelect({
  propertyKey,
  value,
  onChange,
}: PresentationDefinitionSelectProps): ReactElement {
  const {t} = useTranslation();
  const {data, isLoading} = useGetVerifiablePresentations();
  const options = data ?? [];

  return (
    <FormControl fullWidth sx={{mb: 3}}>
      <FormLabel htmlFor={propertyKey}>{t('verifiable-presentations:select.label')}</FormLabel>
      <TextField
        select
        fullWidth
        id={propertyKey}
        value={value ?? ''}
        disabled={isLoading}
        onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        placeholder={t('verifiable-presentations:select.placeholder')}
      >
        {options.map((vp) => (
          <MenuItem key={vp.id} value={vp.handle}>
            {vp.name ?? vp.handle}
          </MenuItem>
        ))}
      </TextField>
    </FormControl>
  );
}
