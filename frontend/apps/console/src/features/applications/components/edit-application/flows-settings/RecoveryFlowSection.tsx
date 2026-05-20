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

import {SettingsCard} from '@thunderid/components';
import {Box, Typography, TextField, Autocomplete, CircularProgress, Alert} from '@wso2/oxygen-ui';
import {useTranslation, Trans} from 'react-i18next';
import {Link} from 'react-router';
import useGetFlows from '../../../../flows/api/useGetFlows';
import {FlowType} from '../../../../flows/models/flows';
import type {Application} from '../../../models/application';

/**
 * Props for the {@link RecoveryFlowSection} component.
 */
interface RecoveryFlowSectionProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * Partial application object containing edited fields
   */
  editedApp: Partial<Application>;
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
}

/**
 * Section component for selecting recovery flow.
 *
 * Provides:
 * - Toggle switch to enable/disable recovery flow
 * - Autocomplete dropdown to select from available recovery flows
 * - Loading state while fetching flows
 *
 * @param props - Component props
 * @returns Recovery flow selection UI within a SettingsCard
 */
export default function RecoveryFlowSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: RecoveryFlowSectionProps) {
  const {t} = useTranslation();
  const {data: recoveryFlowsData, isLoading: loadingRecoveryFlows} = useGetFlows({flowType: FlowType.RECOVERY});

  const recoveryFlowOptions = recoveryFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.recoveryFlow')}
      description={t('applications:edit.flows.labels.recoveryFlow.description')}
      enabled={editedApp.isRecoveryFlowEnabled ?? application.isRecoveryFlowEnabled ?? false}
      onToggle={(enabled) => onFieldChange('isRecoveryFlowEnabled', enabled)}
    >
      {(editedApp.recoveryFlowId ?? application.recoveryFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.recoveryFlow.alert"
            components={[
              <Link
                key="edit"
                to={`/flows/recovery/${editedApp.recoveryFlowId ?? application.recoveryFlowId}`}
                style={{color: 'inherit', fontWeight: 'bold', textDecoration: 'underline'}}
              />,
              <Link
                key="create"
                to="/flows"
                style={{color: 'inherit', fontWeight: 'bold', textDecoration: 'underline'}}
              />,
            ]}
          />
        </Alert>
      )}
      <Autocomplete
        fullWidth
        options={recoveryFlowOptions}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={
          recoveryFlowOptions.find((flow) => flow.id === (editedApp.recoveryFlowId ?? application.recoveryFlowId)) ??
          null
        }
        onChange={(_event, newValue) => onFieldChange('recoveryFlowId', newValue?.id ?? '')}
        loading={loadingRecoveryFlows}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={t('applications:edit.flows.recoveryFlow.placeholder')}
            helperText={t('applications:edit.flows.recoveryFlow.hint', {entity: entityLabel})}
            InputProps={{
              ...params.InputProps,
              endAdornment: (
                <>
                  {loadingRecoveryFlows ? <CircularProgress color="inherit" size={20} /> : null}
                  {params.InputProps.endAdornment}
                </>
              ),
            }}
          />
        )}
        renderOption={(props, option) => (
          <li {...props} key={option.id}>
            <Box>
              <Typography variant="body1">{option.name}</Typography>
              <Typography variant="caption" color="text.secondary">
                {option.handle}
              </Typography>
            </Box>
          </li>
        )}
      />
    </SettingsCard>
  );
}
