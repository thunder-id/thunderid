// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FlowType, useGetFlows} from '@thunderid/configure-flows';
import {Alert, Autocomplete, Box, CircularProgress, TextField, Typography} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import type {Application} from '../../../models/application';
import {OAuth2GrantTypes} from '../../../models/oauth';

interface DeviceGrantFlowSectionProps {
  application: Application;
  editedApp: Partial<Application>;
  onFieldChange: (field: keyof Application, value: unknown) => void;
  entityLabel?: string;
}

const DEFAULT_DEVICE_GRANT_FLOW_HANDLE = 'default-device-grant-flow';

export default function DeviceGrantFlowSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: DeviceGrantFlowSectionProps) {
  const {t} = useTranslation();
  const inboundAuthConfig = editedApp.inboundAuthConfig ?? application.inboundAuthConfig;
  const deviceGrantEnabled = inboundAuthConfig?.some(
    ({type, config}) => type === 'oauth2' && config.grantTypes?.includes(OAuth2GrantTypes.DEVICE_CODE),
  );
  const selectedFlowId = editedApp.deviceGrantFlowId ?? application.deviceGrantFlowId;
  const {data, isLoading} = useGetFlows({flowType: FlowType.DEVICE_GRANT});
  const flows = data?.flows ?? [];
  const selectedFlow = flows.find((flow) => flow.id === selectedFlowId);

  if (!deviceGrantEnabled) return null;

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.deviceGrantFlow', 'Device authorization flow')}
      description={t(
        'applications:edit.flows.labels.deviceGrantFlow.description',
        'Choose the flow used to authorize devices for this {{entity}}.',
        {entity: entityLabel},
      )}
    >
      {selectedFlow?.handle === DEFAULT_DEVICE_GRANT_FLOW_HANDLE && (
        <Alert severity="info" sx={{mb: 2}}>
          {t(
            'applications:edit.flows.deviceGrantFlow.defaultAssigned',
            'The default Device Grant flow is selected and will be saved for this {{entity}}. It verifies the device code and calls this {{entity}}’s Authentication flow. You can select a different flow below.',
            {entity: entityLabel},
          )}
        </Alert>
      )}
      <Autocomplete
        fullWidth
        options={flows}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
        value={selectedFlow ?? null}
        onChange={(_event, newValue) => onFieldChange('deviceGrantFlowId', newValue?.id ?? '')}
        loading={isLoading}
        disabled={application.isReadOnly}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={t('applications:edit.flows.deviceGrantFlow.placeholder', 'Select a Device Grant flow')}
            helperText={t(
              'applications:edit.flows.deviceGrantFlow.hint',
              'This flow verifies the device code and calls this {{entity}}’s Authentication flow.',
              {entity: entityLabel},
            )}
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
