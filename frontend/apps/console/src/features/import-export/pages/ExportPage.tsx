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

import {useLogger} from '@thunderid/logger/react';
import {Alert, Box, CircularProgress, IconButton, LinearProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useExportConfiguration from '../api/useExportConfiguration';
import ConfigureExport from '../components/ConfigureExport';

export default function ExportPage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const navigate = useNavigate();
  const logger = useLogger('ExportPage');
  const {mutate, data, isPending, isError, error} = useExportConfiguration();

  // Fetch export data on component mount
  useEffect(() => {
    mutate({
      applications: ['*'],
      identityProviders: ['*'],
      flows: ['*'],
      themes: ['*'],
      users: ['*'],
      organizationUnits: ['*'],
      notificationSenders: ['*'],
      userTypes: ['*'],
      translations: ['*'],
      layouts: ['*'],
      resourceServers: ['*'],
      roles: ['*'],
      groups: ['*'],
    });
  }, [mutate]);

  // Extract resources and environment variables from API response
  const {resources, environmentVariables} = useMemo(() => {
    if (!data) {
      return {resources: '', environmentVariables: ''};
    }

    return {
      resources: data.resources,
      environmentVariables: data.environment_variables,
    };
  }, [data]);

  const handleClose = (): void => {
    (async () => {
      await navigate(-1);
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate back', {error: _error});
    });
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={100} sx={{height: 6}} />

      <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0}}>
        <Stack direction="row" spacing={2} sx={{alignItems: 'center'}}>
          <IconButton
            aria-label={t('common:actions.close')}
            onClick={handleClose}
            sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
          >
            <X size={24} />
          </IconButton>
          <Typography variant="h5" color="text.primary">
            {t('export.page.title')}
          </Typography>
        </Stack>
      </Box>

      <Box
        sx={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          py: 8,
          px: {xs: 2, sm: 3, md: 8, lg: 20},
          alignItems: 'flex-start',
        }}
      >
        <Box
          sx={{
            width: '100%',
            maxWidth: 1600,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {isPending && (
            <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400}}>
              <Stack spacing={2} sx={{alignItems: 'center'}}>
                <CircularProgress />
                <Typography variant="body2" color="text.secondary">
                  {t('export.page.loading')}
                </Typography>
              </Stack>
            </Box>
          )}

          {isError && (
            <Box sx={{mb: 3}}>
              <Alert severity="error">
                <Typography variant="body2">
                  {t('export.page.loadError', {message: error?.message ?? t('common:dictionary.unknown')})}
                </Typography>
              </Alert>
            </Box>
          )}

          {data && !isPending && <ConfigureExport resources={resources} environmentVariables={environmentVariables} />}
        </Box>
      </Box>
    </Box>
  );
}
