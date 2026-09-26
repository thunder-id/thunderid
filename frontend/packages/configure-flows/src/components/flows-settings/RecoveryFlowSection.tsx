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
  const flowRoutes = useFlowRoutes();
  const {data: recoveryFlowsData, isLoading: loadingRecoveryFlows} = useGetFlows({flowType: FlowType.RECOVERY});

  const recoveryFlowOptions = recoveryFlowsData?.flows ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.recoveryFlow')}
      description={t(
        'applications:edit.flows.labels.recoveryFlow.description',
        'Let people recover their account when signing in through this {{entity}}.',
        {entity: entityLabel},
      )}
      enabled={editedApp.isRecoveryFlowEnabled ?? application.isRecoveryFlowEnabled ?? false}
      onToggle={application.isReadOnly ? undefined : (enabled) => onFieldChange('isRecoveryFlowEnabled', enabled)}
    >
      {(editedApp.recoveryFlowId ?? application.recoveryFlowId) && (
        <Alert severity="info" sx={{mb: 2}}>
          <Trans
            i18nKey="applications:edit.flows.recoveryFlow.alert"
            components={[
              <Link
                key="edit"
                to={flowRoutes.flows.detail(editedApp.recoveryFlowId ?? application.recoveryFlowId ?? '')}
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
        options={recoveryFlowOptions}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={
          recoveryFlowOptions.find((flow) => flow.id === (editedApp.recoveryFlowId ?? application.recoveryFlowId)) ??
          null
        }
        onChange={(_event, newValue) => onFieldChange('recoveryFlowId', newValue?.id ?? '')}
        loading={loadingRecoveryFlows}
        disabled={application.isReadOnly}
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
