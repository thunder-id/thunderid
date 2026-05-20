/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {User} from '@thunderid/react';
import {Box, PageContent, Stack, Typography, useTheme} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import NextStepsSection from '../components/NextStepsSection';
import StartBuildingSection from '../components/StartBuildingSection';

export default function HomePage(): JSX.Element {
  const {t} = useTranslation('home');
  const theme = useTheme();

  return (
    <PageContent>
      <Stack spacing={4}>
        <User>
          {(user: {name?: string; email?: string} | null) => (
            <Box>
              <Typography
                gutterBottom
                variant="h1"
                sx={{
                  fontSize: {xs: '2rem', sm: '2.2rem', md: '2.5rem'},
                  fontWeight: 500,
                }}
              >
                {t('greeting.hello', 'Hello,')}{' '}
                <Box
                  component="span"
                  sx={{
                    background: theme.gradient?.primary,
                    WebkitBackgroundClip: 'text',
                    WebkitTextFillColor: 'transparent',
                    backgroundClip: 'text',
                  }}
                >
                  {user?.name?.split(' ')[0] ?? t('greeting.fallback_name', 'there')}
                </Box>
              </Typography>
              <Typography
                variant="h4"
                sx={{
                  fontSize: {xs: '1rem', sm: '1.1rem', md: '1.2rem'},
                  fontWeight: 400,
                  color: 'text.secondary',
                }}
              >
                {t('greeting.subtitle', 'What do you want to secure today?')}
              </Typography>
            </Box>
          )}
        </User>
        <StartBuildingSection />
        <NextStepsSection />
      </Stack>
    </PageContent>
  );
}
