// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {FlowType, useGetFlows} from '@thunderid/configure-flows';
import {Autocomplete, Box, CircularProgress, TextField, Typography} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import type {Application} from '../../../models/application';
import {OAuth2GrantTypes} from '../../../models/oauth';

interface CibaFlowSectionProps {
  application: Application;
  editedApp: Partial<Application>;
  onFieldChange: (field: keyof Application, value: unknown) => void;
  entityLabel?: string;
  cibaFlowQueryError?: Error | null;
  onRetryCibaFlowQuery?: () => void;
}

export default function CibaFlowSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
  cibaFlowQueryError = undefined,
  onRetryCibaFlowQuery = undefined,
}: CibaFlowSectionProps) {
  const {t} = useTranslation();
  const inboundAuthConfig = editedApp.inboundAuthConfig ?? application.inboundAuthConfig;
  const cibaEnabled = inboundAuthConfig?.some(
    ({type, config}) => type === 'oauth2' && config.grantTypes?.includes(OAuth2GrantTypes.CIBA),
  );
  const selectedFlowId = editedApp.cibaFlowId ?? application.cibaFlowId;
  const {data, isLoading, error, refetch} = useGetFlows({
    flowType: FlowType.CIBA,
    limit: 100,
    fetchAllPages: true,
  });
  const flows = data?.flows ?? [];
  const selectedFlow = flows.find((flow) => flow.id === selectedFlowId);
  const queryError = cibaFlowQueryError ?? error;

  if (!cibaEnabled) return null;

  return (
    <SettingsCard
      title={t('applications:edit.flows.labels.cibaFlow', 'CIBA flow')}
      description={t(
        'applications:edit.flows.labels.cibaFlow.description',
        'Choose the flow that notifies the user and asks for approval of a backchannel authentication request for this {{entity}}.',
        {entity: entityLabel},
      )}
    >
      {queryError ? (
        <QueryErrorNotice
          error={queryError}
          t={t}
          variant="inline"
          onRetry={onRetryCibaFlowQuery ?? (() => void refetch())}
        />
      ) : (
        <Autocomplete
          fullWidth
          options={flows}
          getOptionLabel={(option) => (typeof option === 'string' ? option : option.name)}
          value={selectedFlow ?? null}
          onChange={(_event, newValue) => onFieldChange('cibaFlowId', newValue?.id ?? '')}
          loading={isLoading}
          disabled={application.isReadOnly}
          renderInput={(params) => (
            <TextField
              {...params}
              placeholder={t('applications:edit.flows.cibaFlow.placeholder', 'Select a CIBA flow')}
              helperText={t(
                'applications:edit.flows.cibaFlow.hint',
                'This flow calls this {{entity}}’s Authentication flow once the user approves. When none is selected, the Authentication flow handles the request.',
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
      )}
    </SettingsCard>
  );
}
