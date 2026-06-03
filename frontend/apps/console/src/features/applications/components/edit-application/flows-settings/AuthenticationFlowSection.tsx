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
 * Props for the {@link AuthenticationFlowSection} component.
 */
interface AuthenticationFlowSectionProps {
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
 * Section component for selecting authentication flow.
 *
 * Provides:
 * - Autocomplete dropdown to select from available authentication flows
 * - Link to edit the currently selected flow
 * - Link to create new flows
 * - Loading state while fetching flows
 *
 * @param props - Component props
 * @returns Authentication flow selection UI within a SettingsCard
 */
export default function AuthenticationFlowSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: AuthenticationFlowSectionProps) {
  const {t} = useTranslation();
  const {data: authFlowsData, isLoading: loadingAuthFlows} = useGetFlows({flowType: FlowType.AUTHENTICATION});

  const authFlowOptions = authFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.authFlow')}
      description={t('applications:edit.flows.labels.authFlow.description')}
    >
      {(editedApp.authFlowId ?? application.authFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.authFlow.alert"
            components={[
              <Link
                key="edit"
                to={`/flows/signin/${editedApp.authFlowId ?? application.authFlowId}`}
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
        options={authFlowOptions}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={authFlowOptions.find((flow) => flow.id === (editedApp.authFlowId ?? application.authFlowId)) ?? null}
        onChange={(_event, newValue) => onFieldChange('authFlowId', newValue?.id ?? '')}
        loading={loadingAuthFlows}
        disabled={application.isReadOnly}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={t('applications:edit.flows.authFlow.placeholder')}
            helperText={t(
              'applications:edit.flows.authFlow.hint',
              'Select the flow that handles user sign-in for this {{entity}}.',
              {entity: entityLabel},
            )}
            InputProps={{
              ...params.InputProps,
              endAdornment: (
                <>
                  {loadingAuthFlows ? <CircularProgress color="inherit" size={20} /> : null}
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
