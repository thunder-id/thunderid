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

import {
  Alert,
  Checkbox,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {EXECUTOR_TO_IDP_TYPE_MAP, FEDERATED_EXECUTORS} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import useIdentityProviders from '@/features/connections/api/useIdentityProviders';
import type {IdentityProviderType} from '@/features/connections/models/identity-provider';
import useValidationStatus from '@/features/flows/hooks/useValidationStatus';
import type {StepData} from '@/features/flows/models/steps';

function FederationProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {selectedNotification} = useValidationStatus();
  const {data: identityProviders, isLoading: isLoadingIdps} = useIdentityProviders();

  const executorName = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.action?.executor?.name;
  }, [resource]);

  const currentIdpId = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties as {idpId?: string})?.idpId ?? '';
  }, [resource]);

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties ?? {}) as Record<string, unknown>;
  }, [resource]);

  const idpType: IdentityProviderType | null = useMemo(() => {
    if (!executorName) {
      return null;
    }
    return EXECUTOR_TO_IDP_TYPE_MAP[executorName] ?? null;
  }, [executorName]);

  const availableConnections = useMemo(() => {
    if (!idpType || !identityProviders) {
      return [];
    }

    return identityProviders.filter((idp) => idp.type === idpType);
  }, [idpType, identityProviders]);

  const isPlaceholder = currentIdpId === '{{IDP_ID}}' || currentIdpId === '';

  const errorMessage: string = useMemo(() => {
    const key = `${resource?.id}_data.properties.idpId`;

    if (selectedNotification?.hasResourceFieldNotification(key)) {
      return selectedNotification?.getResourceFieldNotification(key);
    }

    return '';
  }, [resource, selectedNotification]);

  const handleConnectionChange = (selectedIdpId: string): void => {
    onChange('data.properties.idpId', selectedIdpId, resource);
  };

  const handleBooleanPropertyChange = useCallback(
    (propertyName: string, value: boolean): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
    },
    [resource, onChange],
  );

  const isFederatedExecutor = executorName != null && FEDERATED_EXECUTORS.has(executorName);
  const hasConnections = availableConnections.length > 0;
  // Only show the error when the user can actually fix it (connections available but none selected)
  const showError = hasConnections && (isPlaceholder || !!errorMessage);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.federation.connection.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="connection-select">{t('flows:core.executions.federation.connection.label')}</FormLabel>
        <Select
          id="connection-select"
          value={isPlaceholder ? '' : currentIdpId}
          onChange={(e) => handleConnectionChange(e.target.value)}
          displayEmpty
          fullWidth
          error={showError}
          disabled={isLoadingIdps || !hasConnections}
        >
          <MenuItem value="" disabled>
            {isLoadingIdps ? t('common:status.loading') : t('flows:core.executions.federation.connection.placeholder')}
          </MenuItem>
          {availableConnections.map((idp) => (
            <MenuItem key={idp.id} value={idp.id}>
              {idp.name}
            </MenuItem>
          ))}
        </Select>
        {showError && (
          <FormHelperText error>
            {errorMessage || t('flows:core.executions.federation.connection.required')}
          </FormHelperText>
        )}
      </div>

      {!isLoadingIdps && !hasConnections && (
        <Alert severity="warning">{t('flows:core.executions.federation.connection.noConnections')}</Alert>
      )}

      {isFederatedExecutor && (
        <>
          <FormControlLabel
            control={
              <Checkbox
                checked={!!properties.allowAuthenticationWithoutLocalUser}
                onChange={(e) => handleBooleanPropertyChange('allowAuthenticationWithoutLocalUser', e.target.checked)}
                size="small"
              />
            }
            label={t('flows:core.executions.federation.allowAuthenticationWithoutLocalUser.label')}
          />
          <FormHelperText>
            {t('flows:core.executions.federation.allowAuthenticationWithoutLocalUser.hint')}
          </FormHelperText>

          <FormControlLabel
            control={
              <Checkbox
                checked={!!properties.allowRegistrationWithExistingUser}
                onChange={(e) => handleBooleanPropertyChange('allowRegistrationWithExistingUser', e.target.checked)}
                size="small"
              />
            }
            label={t('flows:core.executions.federation.allowRegistrationWithExistingUser.label')}
          />
          <FormHelperText>
            {t('flows:core.executions.federation.allowRegistrationWithExistingUser.hint')}
          </FormHelperText>

          <FormControlLabel
            control={
              <Checkbox
                checked={!!properties.allowCrossOUProvisioning}
                onChange={(e) => handleBooleanPropertyChange('allowCrossOUProvisioning', e.target.checked)}
                size="small"
              />
            }
            label={t('flows:core.executions.federation.allowCrossOUProvisioning.label')}
          />
          <FormHelperText>{t('flows:core.executions.federation.allowCrossOUProvisioning.hint')}</FormHelperText>
        </>
      )}
    </Stack>
  );
}

export default FederationProperties;
