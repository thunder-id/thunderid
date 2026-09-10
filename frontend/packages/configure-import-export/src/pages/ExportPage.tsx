// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {AppBreadcrumbs, Box, CircularProgress, IconButton, LinearProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback, useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useExportConfiguration from '../api/useExportConfiguration';
import ConfigureExport from '../components/ConfigureExport';
import useImportExportRoutes from '../hooks/useImportExportRoutes';

export default function ExportPage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const navigate = useNavigate();
  const logger = useLogger('ExportPage');
  const routes = useImportExportRoutes();
  const {mutate, data, isPending, error} = useExportConfiguration();

  const fetchExport = useCallback((): void => {
    mutate({
      applications: ['*'],
      connections: ['*'],
      flows: ['*'],
      themes: ['*'],
      users: ['*'],
      organizationUnits: ['*'],
      userTypes: ['*'],
      agentTypes: ['*'],
      translations: ['*'],
      layouts: ['*'],
      resourceServers: ['*'],
      roles: ['*'],
      groups: ['*'],
      agents: ['*'],
      serverConfigs: ['*'],
      credentialConfigurations: ['*'],
      presentationDefinitions: ['*'],
    });
  }, [mutate]);

  // Fetch export data on component mount
  useEffect(() => {
    fetchExport();
  }, [fetchExport]);

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
          <AppBreadcrumbs
            items={[
              {
                key: 'import-export',
                label: t('landing.title', 'Import / Export'),
                onClick: () => void navigate(routes.importExport.list()),
              },
              {key: 'export', label: t('export.page.title')},
            ]}
          />
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

          {error && (
            <Box sx={{mb: 3}}>
              <QueryErrorNotice
                error={error}
                t={t}
                variant="block"
                title={t('export.page.title')}
                fallbackKey="export.page.loadError"
                fallbackDefaultValue="Failed to load export configuration"
                onRetry={fetchExport}
              />
            </Box>
          )}

          {data && !isPending && <ConfigureExport resources={resources} environmentVariables={environmentVariables} />}
        </Box>
      </Box>
    </Box>
  );
}
