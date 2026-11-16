// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useCapabilitiesContext, useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {AppBreadcrumbs, Box, Card, CardContent, IconButton, LinearProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {FileOutput, FileInput, X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useImportExportRoutes from '../hooks/useImportExportRoutes';

interface ImportExportOption {
  /** Whether the admin may do what the option opens. */
  allowed: boolean;
  route: string;
  labelKey: string;
  labelDefault: string;
  descriptionKey: string;
  descriptionDefault: string;
  icon: JSX.Element;
}

export default function ImportExportPage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const {config} = useConfig();
  const productName: string = config.brand.product_name;
  const navigate = useNavigate();
  const logger = useLogger('ImportExportPage');
  const routes = useImportExportRoutes();
  const {can} = useCapabilitiesContext();

  const handleClose = (): void => {
    (async () => {
      await navigate(routes.home.list());
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to home page', {error});
    });
  };

  const handleSelect = (route: string): void => {
    (async () => {
      await navigate(route);
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to import/export sub-page', {error, route});
    });
  };

  const options: ImportExportOption[] = [
    {
      allowed: can('import', 'create'),
      route: routes.importConfiguration.upload(),
      labelKey: 'landing.type.import.label',
      labelDefault: 'Import',
      descriptionKey: 'landing.type.import.description',
      descriptionDefault: 'Bring in an existing {{productName}} configuration file.',
      icon: <FileInput size={28} />,
    },
    {
      allowed: can('export', 'create'),
      route: routes.export.page(),
      labelKey: 'landing.type.export.label',
      labelDefault: 'Export',
      descriptionKey: 'landing.type.export.description',
      descriptionDefault: 'Download your current configuration as a file.',
      icon: <FileOutput size={28} />,
    },
  ].filter((option: ImportExportOption) => option.allowed);

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <DocumentTitle title={t('landing.title', 'Import / Export')} />
      <LinearProgress variant="determinate" value={0} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Stack direction="row" alignItems="center" spacing={2}>
            <IconButton
              aria-label={t('common:actions.close', 'Close')}
              onClick={handleClose}
              sx={{
                bgcolor: 'background.paper',
                '&:hover': {bgcolor: 'action.hover'},
                boxShadow: 1,
              }}
            >
              <X size={24} />
            </IconButton>
            <AppBreadcrumbs items={[{key: 'import-export', label: t('landing.title', 'Import / Export')}]} />
          </Stack>
        </Box>

        <Box sx={{flex: 1, display: 'flex', minHeight: 0}}>
          <Box
            sx={{
              flex: 1,
              display: 'flex',
              flexDirection: 'column',
              pt: 8,
              pb: 8,
              px: 20,
              mx: 'auto',
              alignItems: 'flex-start',
              justifyContent: 'flex-start',
            }}
          >
            <Box sx={{width: '100%', maxWidth: 800, display: 'flex', flexDirection: 'column'}}>
              <Stack direction="column" spacing={4} data-testid="import-export-type-select">
                <Typography variant="h1" gutterBottom>
                  {t('landing.title', 'Import / Export')}
                </Typography>
                <Typography variant="body1" color="text.secondary">
                  {t('landing.subtitle', 'Choose whether to import a configuration file or export your current one.')}{' '}
                  <ExternalLink docKey="importExport" confirmBeforeNavigate={false} />
                </Typography>
                <Box
                  sx={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(2, 1fr)',
                    maxWidth: 560,
                    gap: 2,
                  }}
                >
                  {options.map((option) => (
                    <Card
                      key={option.route}
                      variant="outlined"
                      role="button"
                      tabIndex={0}
                      onClick={() => handleSelect(option.route)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          handleSelect(option.route);
                        }
                      }}
                      sx={{
                        cursor: 'pointer',
                        borderColor: 'divider',
                        transition: 'border-color 0.15s',
                        '&:hover': {borderColor: 'primary.main'},
                        '&:focus-visible': {outline: 'none', borderColor: 'primary.main'},
                      }}
                    >
                      <CardContent sx={{py: 2, px: 2}}>
                        <Stack direction="column" spacing={1.5} alignItems="flex-start">
                          <Box sx={{color: 'text.secondary'}}>{option.icon}</Box>
                          <Stack direction="column" spacing={0.5}>
                            <Typography variant="subtitle1" sx={{fontWeight: 500}}>
                              {t(option.labelKey, option.labelDefault)}
                            </Typography>
                            <Typography variant="body2" color="text.secondary">
                              {t(option.descriptionKey, option.descriptionDefault, {productName})}
                            </Typography>
                          </Stack>
                        </Stack>
                      </CardContent>
                    </Card>
                  ))}
                </Box>
              </Stack>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
