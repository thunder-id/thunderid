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

import {useIdentityProviders, type IdentityProviderType} from '@thunderid/configure-connections';
import {Alert, FormHelperText, FormLabel, MenuItem, Select, Stack} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import {EXECUTOR_TO_IDP_TYPE_MAP, FEDERATED_EXECUTORS} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import useResourceFieldError from '@/features/flows/hooks/useResourceFieldError';
import type {StepData} from '@/features/flows/models/steps';

function FederationProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
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
    return stepData?.properties ?? {};
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

  const errorMessage: string = useResourceFieldError(resource?.id, 'data.properties.idpId');

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
        <Stack gap={1}>
          <CheckboxWithHint
            checked={!!properties.allowAuthenticationWithoutLocalUser}
            onChange={(checked) => handleBooleanPropertyChange('allowAuthenticationWithoutLocalUser', checked)}
            label={t(
              'flows:core.executions.federation.allowAuthenticationWithoutLocalUser.label',
              'Allow Authentication Without Local User',
            )}
            hint={t(
              'flows:core.executions.federation.allowAuthenticationWithoutLocalUser.hint',
              'Allow users to authenticate even when no matching local user exists.',
            )}
          />
          <CheckboxWithHint
            checked={!!properties.allowRegistrationWithExistingUser}
            onChange={(checked) => handleBooleanPropertyChange('allowRegistrationWithExistingUser', checked)}
            label={t(
              'flows:core.executions.federation.allowRegistrationWithExistingUser.label',
              'Allow Registration With Existing User',
            )}
            hint={t(
              'flows:core.executions.federation.allowRegistrationWithExistingUser.hint',
              'Allow existing users to proceed through registration flows.',
            )}
          />
          <CheckboxWithHint
            checked={!!properties.allowCrossOUProvisioning}
            onChange={(checked) => handleBooleanPropertyChange('allowCrossOUProvisioning', checked)}
            label={t('flows:core.executions.federation.allowCrossOUProvisioning.label', 'Allow Cross-OU Provisioning')}
            hint={t(
              'flows:core.executions.federation.allowCrossOUProvisioning.hint',
              'Allow creating a user in a different organizational unit.',
            )}
          />
        </Stack>
      )}
    </Stack>
  );
}

export default FederationProperties;
