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

import {Box, Card, CardContent, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ResourceServerTypeMetadataList from '../../config/resource-server-types';
import type {ResourceServerType} from '../../models/resource-server';

interface ConfigureTypeProps {
  selectedType: ResourceServerType | undefined;
  onSelect: (value: ResourceServerType) => void;
}

export default function ConfigureType({selectedType, onSelect}: ConfigureTypeProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack direction="column" spacing={3}>
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h1">
          {t('resourceServers:create.type.title', 'What type of resource server are you adding?')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t('resourceServers:create.type.subtitle', 'Select the type that best describes this resource server.')}
        </Typography>
      </Stack>

      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {
            xs: '1fr',
            sm: 'repeat(2, 1fr)',
            md: 'repeat(3, 1fr)',
          },
          gap: 2,
        }}
      >
        {ResourceServerTypeMetadataList.map((option) => {
          const isSelected = selectedType === option.value;

          return (
            <Card
              key={option.value}
              variant="outlined"
              role="button"
              tabIndex={0}
              aria-pressed={isSelected}
              onClick={() => onSelect(option.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  onSelect(option.value);
                }
              }}
              sx={{
                borderRadius: 2,
                borderWidth: isSelected ? 2 : 1,
                borderColor: isSelected ? 'primary.main' : 'divider',
                cursor: 'pointer',
                bgcolor: isSelected ? 'action.selected' : 'background.paper',
                transition: 'border-color 0.15s, box-shadow 0.15s, transform 0.15s',
                '&:hover': {
                  borderColor: 'primary.main',
                  boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
                  transform: 'translateY(-2px)',
                },
                '&:focus-visible': {
                  outline: 'none',
                  borderColor: 'primary.main',
                  boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
                  transform: 'translateY(-2px)',
                },
              }}
            >
              <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
                <Stack direction="column" spacing={2}>
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      width: 48,
                      height: 48,
                    }}
                  >
                    {option.icon}
                  </Box>
                  <Stack direction="column" spacing={0.75}>
                    <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
                      {t(option.titleKey)}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
                      {t(option.descriptionKey)}
                    </Typography>
                  </Stack>
                </Stack>
              </CardContent>
            </Card>
          );
        })}
      </Box>
    </Stack>
  );
}
