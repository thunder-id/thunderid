// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import type {Application} from '@thunderid/configure-applications';
import {Box, Typography, TextField, Autocomplete, CircularProgress, Alert} from '@wso2/oxygen-ui';
import {useTranslation, Trans} from 'react-i18next';
import {Link} from 'react-router';
import useGetFlows from '../../api/useGetFlows';
import useFlowRoutes from '../../hooks/useFlowRoutes';
import {FlowType} from '../../models/flows';

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
  const flowRoutes = useFlowRoutes();
  const {data: signoutFlowsData, isLoading: loadingSignOutFlows} = useGetFlows({flowType: FlowType.SIGNOUT});

  const signoutFlowOptions = signoutFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.signOutFlow', 'Sign Out Flow')}
      description={t(
        'applications:edit.flows.labels.signOutFlow.description',
        'Terminate the SSO session when people sign out of this {{entity}}.',
        {entity: entityLabel},
      )}
    >
      {(editedApp.signOutFlowId ?? application.signOutFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.signOutFlow.alert"
            components={[
              <Link
                key="edit"
                to={flowRoutes.flows.detail(editedApp.signOutFlowId ?? application.signOutFlowId ?? '')}
                style={{color: 'inherit', fontWeight: 'bold', textDecoration: 'underline'}}
              />,
              <Link
                key="create"
                to={flowRoutes.flows.list()}
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
            placeholder={t('applications:edit.flows.signOutFlow.placeholder', 'Select a sign-out flow')}
            helperText={t(
              'applications:edit.flows.signOutFlow.hint',
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
