// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import type {DefaultFlowsSettingsRenderProps} from '@thunderid/configure-organization-units';
import {Box, Typography, TextField, Autocomplete, CircularProgress, Stack, Alert} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useGetFlows from '../api/useGetFlows';
import useFlowRoutes from '../hooks/useFlowRoutes';
import {FlowType} from '../models/flows';

type DefaultFlowField = 'authFlowId' | 'registrationFlowId' | 'recoveryFlowId' | 'signOutFlowId';
type DefaultFlowEnabledField = 'isRegistrationFlowEnabled' | 'isRecoveryFlowEnabled' | 'isSignOutFlowEnabled';

interface DefaultFlowSectionProps extends DefaultFlowsSettingsRenderProps {
  flowType: FlowType;
  field: DefaultFlowField;
  /** When provided, the section renders an enable/disable toggle bound to this field. */
  enabledField?: DefaultFlowEnabledField;
  /** Already-translated title, description, placeholder, and helper text. */
  title: string;
  description: string;
  placeholder: string;
  hint: string;
  /** i18n key for the "open the flow builder" / "Flows page" info banner shown when a flow is selected. */
  alertI18nKey: string;
}

/** A single "default X flow" picker card, reused for each flow type below. */
function DefaultFlowSection({
  organizationUnit,
  editedOU,
  onFieldChange,
  flowType,
  field,
  enabledField = undefined,
  title,
  description,
  placeholder,
  hint,
  alertI18nKey,
}: DefaultFlowSectionProps): JSX.Element {
  const flowRoutes = useFlowRoutes();
  const {data: flowsData, isLoading} = useGetFlows({flowType});
  const flowOptions = flowsData?.flows ?? [];
  const selectedFlowId = editedOU[field] ?? organizationUnit[field];

  return (
    <SettingsCard
      title={title}
      description={description}
      enabled={enabledField ? (editedOU[enabledField] ?? organizationUnit[enabledField] ?? false) : undefined}
      onToggle={
        enabledField && !organizationUnit.isReadOnly ? (enabled) => onFieldChange(enabledField, enabled) : undefined
      }
    >
      {selectedFlowId && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey={alertI18nKey}
            defaults="To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>."
            components={[
              <Link
                key="edit"
                to={flowRoutes.flows.detail(selectedFlowId)}
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
        options={flowOptions}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={flowOptions.find((flow) => flow.id === selectedFlowId) ?? null}
        onChange={(_event, newValue) => onFieldChange(field, newValue?.id ?? '')}
        loading={isLoading}
        disabled={organizationUnit.isReadOnly}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={placeholder}
            helperText={hint}
            InputProps={{
              ...params.InputProps,
              endAdornment: (
                <>
                  {isLoading ? <CircularProgress color="inherit" size={20} /> : null}
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

/**
 * Default flows tab for the organization unit edit page, injected via
 * `OrganizationUnitEditPage`'s `renderDefaultFlowsSettings` prop since the OU package can't depend
 * on this package's flow-fetching API. Lets an OU define default Sign In, Sign Up,
 * Recovery, and Sign Out flows that new applications can snapshot at creation time.
 */
export default function OrganizationUnitDefaultFlowsSettings(
  renderProps: DefaultFlowsSettingsRenderProps,
): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3}>
      <DefaultFlowSection
        {...renderProps}
        flowType={FlowType.AUTHENTICATION}
        field="authFlowId"
        title={t('organizationUnits:edit.flows.labels.authenticationFlow', 'Sign-in Flow')}
        description={t(
          'organizationUnits:edit.flows.authenticationFlow.description',
          'Choose the default flow that handles user login and authentication for applications under this organization unit.',
        )}
        placeholder={t('organizationUnits:edit.flows.authenticationFlow.placeholder', 'Select an authentication flow')}
        hint={t(
          'organizationUnits:edit.flows.authenticationFlow.hint',
          'Select the flow that handles user sign-in for applications under this organization unit.',
        )}
        alertI18nKey="organizationUnits:edit.flows.authenticationFlow.alert"
      />
      <DefaultFlowSection
        {...renderProps}
        flowType={FlowType.REGISTRATION}
        field="registrationFlowId"
        enabledField="isRegistrationFlowEnabled"
        title={t('organizationUnits:edit.flows.labels.registrationFlow', 'Sign-up Flow')}
        description={t(
          'organizationUnits:edit.flows.registrationFlow.description',
          'Choose the default flow that handles user sign-up and account creation for applications under this organization unit.',
        )}
        placeholder={t('organizationUnits:edit.flows.registrationFlow.placeholder', 'Select a registration flow')}
        hint={t(
          'organizationUnits:edit.flows.registrationFlow.hint',
          'Select the flow that handles user registration for applications under this organization unit.',
        )}
        alertI18nKey="organizationUnits:edit.flows.registrationFlow.alert"
      />
      <DefaultFlowSection
        {...renderProps}
        flowType={FlowType.RECOVERY}
        field="recoveryFlowId"
        enabledField="isRecoveryFlowEnabled"
        title={t('organizationUnits:edit.flows.labels.recoveryFlow', 'Recovery Flow')}
        description={t(
          'organizationUnits:edit.flows.recoveryFlow.description',
          'Choose the default flow that handles password and account recovery for applications under this organization unit.',
        )}
        placeholder={t('organizationUnits:edit.flows.recoveryFlow.placeholder', 'Select a recovery flow')}
        hint={t(
          'organizationUnits:edit.flows.recoveryFlow.hint',
          'Select the flow that handles account recovery for applications under this organization unit.',
        )}
        alertI18nKey="organizationUnits:edit.flows.recoveryFlow.alert"
      />
      <DefaultFlowSection
        {...renderProps}
        flowType={FlowType.SIGNOUT}
        field="signOutFlowId"
        title={t('organizationUnits:edit.flows.labels.signOutFlow', 'Sign Out Flow')}
        description={t(
          'organizationUnits:edit.flows.signOutFlow.description',
          'Choose the default flow that handles user sign-out and session termination for applications under this organization unit.',
        )}
        placeholder={t('organizationUnits:edit.flows.signOutFlow.placeholder', 'Select a sign-out flow')}
        hint={t(
          'organizationUnits:edit.flows.signOutFlow.hint',
          'Select the flow that runs when a user signs out of applications under this organization unit.',
        )}
        alertI18nKey="organizationUnits:edit.flows.signOutFlow.alert"
      />
    </Stack>
  );
}
