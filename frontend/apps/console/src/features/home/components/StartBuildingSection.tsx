/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig} from '@thunderid/contexts';
import {Box, Button, Card, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import HomeFloatingLogos from './HomeFloatingLogos';
import useGetApplications from '../../applications/api/useGetApplications';

export default function StartBuildingSection(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation('home');
  const {data} = useGetApplications({limit: 1});
  const {config} = useConfig();

  const {brand} = config;
  const {product_name: productName} = brand || {};
  const totalApps = data?.totalResults ?? 0;
  const hasApps = totalApps > 0;

  return (
    <Box
      component={motion.div}
      initial={{opacity: 0, y: 20}}
      animate={{opacity: 1, y: 0}}
      transition={{duration: 0.4, ease: 'easeOut'}}
    >
      <Card variant="outlined" sx={{position: 'relative', overflow: 'hidden', minHeight: 180, p: 4}}>
        <HomeFloatingLogos />

        {/* Foreground content */}
        <Box sx={{position: 'relative', zIndex: 1, maxWidth: {xs: '100%', sm: '55%'}}}>
          <Stack spacing={2}>
            <Typography variant="h3" fontWeight={600}>
              {t('start_building.hero.title', {
                product: productName,
                defaultValue: 'Integrate {{product}} into your application',
              })}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t(
                'start_building.hero.description',
                'Add secure login, token management, and user sessions to your app in minutes.',
              )}
            </Typography>
            <Stack direction="row" spacing={1.5} alignItems="center">
              {hasApps ? (
                <>
                  <Button
                    variant="contained"
                    size="small"
                    onClick={() => {
                      navigate('/applications/types')?.catch(() => undefined);
                    }}
                    sx={{textTransform: 'none'}}
                  >
                    {t('start_building.hero.actions.view_apps.label', 'Create Applications')}
                  </Button>
                  <Chip
                    label={t('start_building.hero.status.app_count', {
                      count: totalApps,
                      defaultValue: '{{count}} application',
                    })}
                    size="small"
                    variant="outlined"
                    sx={{height: 24, fontSize: '0.75rem'}}
                  />
                </>
              ) : (
                <Button
                  variant="contained"
                  size="small"
                  onClick={() => {
                    navigate('/applications/types')?.catch(() => undefined);
                  }}
                  sx={{textTransform: 'none'}}
                >
                  {t('start_building.hero.actions.create.label', 'Create Application')}
                </Button>
              )}
            </Stack>
          </Stack>
        </Box>
      </Card>
    </Box>
  );
}
