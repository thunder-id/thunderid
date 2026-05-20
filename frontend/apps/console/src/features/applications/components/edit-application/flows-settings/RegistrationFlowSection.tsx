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
import {Box, Typography, TextField, Autocomplete, CircularProgress, Alert} from '@wso2/oxygen-ui';
import {useTranslation, Trans} from 'react-i18next';
import {Link} from 'react-router';
import useGetFlows from '../../../../flows/api/useGetFlows';
import {FlowType} from '../../../../flows/models/flows';
import type {Application} from '../../../models/application';

/**
 * Props for the {@link RegistrationFlowSection} component.
 */
interface RegistrationFlowSectionProps {
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
 * Section component for selecting registration flow.
 *
 * Provides:
 * - Toggle switch to enable/disable registration flow
 * - Autocomplete dropdown to select from available registration flows
 * - Link to edit the currently selected flow
 * - Link to create new flows
 * - Loading state while fetching flows
 *
 * @param props - Component props
 * @returns Registration flow selection UI within a SettingsCard
 */
export default function RegistrationFlowSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: RegistrationFlowSectionProps) {
  const {t} = useTranslation();
  const {data: regFlowsData, isLoading: loadingRegFlows} = useGetFlows({flowType: FlowType.REGISTRATION});

  const regFlowOptions = regFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.registrationFlow')}
      description={t('applications:edit.flows.labels.registrationFlow.description')}
      enabled={editedApp.isRegistrationFlowEnabled ?? application.isRegistrationFlowEnabled ?? false}
      onToggle={(enabled) => onFieldChange('isRegistrationFlowEnabled', enabled)}
    >
      {(editedApp.registrationFlowId ?? application.registrationFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.registrationFlow.alert"
            components={[
              <Link
                key="edit"
                to={`/flows/registration/${editedApp.registrationFlowId ?? application.registrationFlowId}`}
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
        options={regFlowOptions}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={
          regFlowOptions.find((flow) => flow.id === (editedApp.registrationFlowId ?? application.registrationFlowId)) ??
          null
        }
        onChange={(_event, newValue) => onFieldChange('registrationFlowId', newValue?.id ?? '')}
        loading={loadingRegFlows}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={t('applications:edit.flows.registrationFlow.placeholder')}
            helperText={t(
              'applications:edit.flows.registrationFlow.hint',
              'Select the flow that handles user registration for this {{entity}}.',
              {entity: entityLabel},
            )}
            InputProps={{
              ...params.InputProps,
              endAdornment: (
                <>
                  {loadingRegFlows ? <CircularProgress color="inherit" size={20} /> : null}
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
