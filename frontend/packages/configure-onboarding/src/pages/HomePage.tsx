// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {User} from '@thunderid/react';
import {Box, PageContent, Stack, Typography, useTheme} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import NextStepsSection from '../components/home/NextStepsSection';
import StartBuildingSection from '../components/home/StartBuildingSection';

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
