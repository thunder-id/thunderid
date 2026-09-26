// Copyright 2025 The ThunderID Authors
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
  const flowRoutes = useFlowRoutes();
  const {data: authFlowsData, isLoading: loadingAuthFlows} = useGetFlows({flowType: FlowType.AUTHENTICATION});

  const authFlowOptions = authFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.authFlow')}
      description={t(
        'applications:edit.flows.labels.authFlow.description',
        'The flow used when someone signs in through this {{entity}}.',
        {entity: entityLabel},
      )}
    >
      {(editedApp.authFlowId ?? application.authFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.authFlow.alert"
            components={[
              <Link
                key="edit"
                to={flowRoutes.flows.detail(editedApp.authFlowId ?? application.authFlowId ?? '')}
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
