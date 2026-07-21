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
 * Props for the {@link SignOutFlowSection} component.
 */
interface SignOutFlowSectionProps {
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
 * Section component for selecting the signout flow.
 *
 * Provides:
 * - Toggle switch to enable/disable signout
 * - Autocomplete dropdown to select from available signout flows
 * - Loading state while fetching flows
 *
 * @param props - Component props
 * @returns SignOut flow selection UI within a SettingsCard
 */
export default function SignOutFlowSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: SignOutFlowSectionProps) {
  const {t} = useTranslation();
  const {data: signoutFlowsData, isLoading: loadingSignOutFlows} = useGetFlows({flowType: FlowType.SIGNOUT});

  const signoutFlowOptions = signoutFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.signoutFlow', 'Sign Out')}
      description={t(
        'applications:edit.flows.labels.signoutFlow.description',
        'Confirm and terminate the SSO session when people sign out of this {{entity}}.',
        {entity: entityLabel},
      )}
      enabled={editedApp.isSignOutFlowEnabled ?? application.isSignOutFlowEnabled ?? false}
      onToggle={application.isReadOnly ? undefined : (enabled) => onFieldChange('isSignOutFlowEnabled', enabled)}
    >
      {(editedApp.signOutFlowId ?? application.signOutFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.signoutFlow.alert"
            defaults="Edit the <0>selected signout flow</0> or <1>create a new one</1>."
            components={[
              <Link
                key="edit"
                to={`/flows/signout/${editedApp.signOutFlowId ?? application.signOutFlowId}`}
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
        options={signoutFlowOptions}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={
          signoutFlowOptions.find((flow) => flow.id === (editedApp.signOutFlowId ?? application.signOutFlowId)) ?? null
        }
        onChange={(_event, newValue) => onFieldChange('signOutFlowId', newValue?.id ?? '')}
        loading={loadingSignOutFlows}
        disabled={application.isReadOnly}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={t('applications:edit.flows.signoutFlow.placeholder', 'Select a signout flow')}
            helperText={t(
              'applications:edit.flows.signoutFlow.hint',
              'The flow that runs when a user logs out of this {{entity}}.',
              {entity: entityLabel},
            )}
            InputProps={{
              ...params.InputProps,
              endAdornment: (
                <>
                  {loadingSignOutFlows ? <CircularProgress color="inherit" size={20} /> : null}
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
