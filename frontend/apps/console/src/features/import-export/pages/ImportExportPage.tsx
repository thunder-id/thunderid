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
import {
  AppBreadcrumbs,
  Box,
  Card,
  CardActionArea,
  CardContent,
  IconButton,
  LinearProgress,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {FileOutput, FileInput, X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';

interface ImportExportOption {
  route: string;
  labelKey: string;
  labelDefault: string;
  descriptionKey: string;
  descriptionDefault: string;
  icon: JSX.Element;
}

export default function ImportExportPage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const navigate = useNavigate();
  const logger = useLogger('ImportExportPage');

  const handleClose = (): void => {
    (async () => {
      await navigate('/home');
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
      route: '/import-configuration',
      labelKey: 'landing.type.import.label',
      labelDefault: 'Import',
      descriptionKey: 'landing.type.import.description',
      descriptionDefault: 'Bring in an existing ThunderID configuration file.',
      icon: <FileInput size={28} />,
    },
    {
      route: '/export',
      labelKey: 'landing.type.export.label',
      labelDefault: 'Export',
      descriptionKey: 'landing.type.export.description',
      descriptionDefault: 'Download your current configuration as a file.',
      icon: <FileOutput size={28} />,
    },
  ];

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
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
                  {t('landing.subtitle', 'Choose whether to import a configuration file or export your current one.')}
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
                    <Card key={option.route} variant="outlined">
                      <CardActionArea
                        onClick={() => handleSelect(option.route)}
                        sx={{
                          height: '100%',
                          border: 1,
                          borderColor: 'divider',
                          transition: 'all 0.2s ease-in-out',
                          '&:hover': {
                            borderColor: 'primary.main',
                            bgcolor: 'action.hover',
                          },
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
                                {t(option.descriptionKey, option.descriptionDefault)}
                              </Typography>
                            </Stack>
                          </Stack>
                        </CardContent>
                      </CardActionArea>
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
