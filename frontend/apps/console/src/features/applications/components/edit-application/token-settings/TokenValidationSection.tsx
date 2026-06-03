/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {SettingsCard} from '@thunderid/components';
import {Box, FormControl, FormLabel, Stack, Tab, Tabs, TextField} from '@wso2/oxygen-ui';
import {useState} from 'react';
import {Controller} from 'react-hook-form';
import type {Control, FieldErrors} from 'react-hook-form';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link TokenValidationSection} component.
 */
interface TokenValidationSectionProps {
  /**
   * React Hook Form control object for the token config form
   */
  control: Control<{
    validityPeriod: number;
    accessTokenValidity: number;
    idTokenValidity: number;
    refreshTokenValidity: number;
  }>;
  /**
   * Form validation errors
   */
  errors: FieldErrors<{
    validityPeriod: number;
    accessTokenValidity: number;
    idTokenValidity: number;
    refreshTokenValidity: number;
  }>;
  /**
   * Token mode:
   * - 'shared': Single validity period for native apps (no tabs)
   * - 'oauth': Tabbed layout with separate Access Token, ID Token, and Refresh Token validity inputs
   */
  tokenType: 'shared' | 'oauth';
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

/**
 * Section component for configuring token validity periods.
 *
 * - `tokenType="shared"`: renders a single validity period input for native apps.
 * - `tokenType="oauth"`: renders token tabs (Access Token / ID Token / Refresh Token), each with its
 *   own validity period input. The tab state is managed internally and is independent
 *   of any other tab state in the page.
 *
 * @param props - Component props
 * @returns Token validity configuration UI within a SettingsCard
 */
export default function TokenValidationSection({
  control,
  errors,
  tokenType,
  disabled = false,
}: TokenValidationSectionProps) {
  const {t} = useTranslation();
  const [activeValidationTab, setActiveValidationTab] = useState<'access' | 'id' | 'refresh'>('access');

  const title = t('applications:edit.token.token_validation.title', 'Token Validity');
  const description = t(
    'applications:edit.token.token_validation.description',
    'Configure how long tokens remain valid before expiration',
  );
  const label = t('applications:edit.token.labels.token_validity', 'Token Validity');
  const hint = t('applications:edit.token.validity.hint', 'Token validity period in seconds (e.g., 3600 for 1 hour)');

  const renderField = (
    fieldName: 'validityPeriod' | 'accessTokenValidity' | 'idTokenValidity' | 'refreshTokenValidity',
  ) => (
    <FormControl fullWidth required>
      <FormLabel htmlFor={`${fieldName}-input`}>{label}</FormLabel>
      <Controller
        name={fieldName}
        control={control}
        render={({field}) => (
          <TextField
            id={`${fieldName}-input`}
            {...field}
            fullWidth
            type="number"
            onChange={(e) => field.onChange(parseInt(e.target.value, 10))}
            error={!!errors[fieldName]}
            helperText={errors[fieldName]?.message ?? hint}
            inputProps={{min: 1}}
            disabled={disabled}
          />
        )}
      />
    </FormControl>
  );

  if (tokenType === 'oauth') {
    return (
      <SettingsCard slotProps={{content: {sx: {p: 0}}}} title={title} description={description}>
        <Stack spacing={2}>
          <Tabs
            value={activeValidationTab === 'access' ? 0 : activeValidationTab === 'id' ? 1 : 2}
            onChange={(_, newValue: number) => {
              const nextTab = newValue === 0 ? 'access' : newValue === 1 ? 'id' : 'refresh';
              setActiveValidationTab(nextTab);
            }}
            sx={{borderBottom: 1, borderColor: 'divider'}}
          >
            <Tab label={t('applications:edit.token.tabs.access_token', 'Access Token')} />
            <Tab label={t('applications:edit.token.tabs.id_token', 'ID Token')} />
            <Tab label={t('applications:edit.token.tabs.refresh_token', 'Refresh Token')} />
          </Tabs>
          <Box sx={{p: 3}}>
            {activeValidationTab === 'access' && renderField('accessTokenValidity')}
            {activeValidationTab === 'id' && renderField('idTokenValidity')}
            {activeValidationTab === 'refresh' && renderField('refreshTokenValidity')}
          </Box>
        </Stack>
      </SettingsCard>
    );
  }

  return (
    <SettingsCard title={title} description={description}>
      {renderField('validityPeriod')}
    </SettingsCard>
  );
}
