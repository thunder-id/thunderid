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
  const flowRoutes = useFlowRoutes();
  const {data: regFlowsData, isLoading: loadingRegFlows} = useGetFlows({flowType: FlowType.REGISTRATION});

  const regFlowOptions = regFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.registrationFlow')}
      description={t(
        'applications:edit.flows.labels.registrationFlow.description',
        'Let people sign themselves up through this {{entity}}.',
        {entity: entityLabel},
      )}
      enabled={editedApp.isRegistrationFlowEnabled ?? application.isRegistrationFlowEnabled ?? false}
      onToggle={application.isReadOnly ? undefined : (enabled) => onFieldChange('isRegistrationFlowEnabled', enabled)}
    >
      {(editedApp.registrationFlowId ?? application.registrationFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.registrationFlow.alert"
            components={[
              <Link
                key="edit"
                to={flowRoutes.flows.detail(editedApp.registrationFlowId ?? application.registrationFlowId ?? '')}
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
        options={regFlowOptions}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={
          regFlowOptions.find((flow) => flow.id === (editedApp.registrationFlowId ?? application.registrationFlowId)) ??
          null
        }
        onChange={(_event, newValue) => onFieldChange('registrationFlowId', newValue?.id ?? '')}
        loading={loadingRegFlows}
        disabled={application.isReadOnly}
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
