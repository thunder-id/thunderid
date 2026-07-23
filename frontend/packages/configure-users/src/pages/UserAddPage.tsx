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
import {Send, UserPlus, X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useUserRoutes from '../hooks/useUserRoutes';

interface AddUserOption {
  route: string;
  labelKey: string;
  labelDefault: string;
  descriptionKey: string;
  descriptionDefault: string;
  icon: JSX.Element;
}

export default function UserAddPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('UserAddPage');
  const routes = useUserRoutes();

  const handleClose = (): void => {
    (async () => {
      await navigate(routes.list());
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to users page', {error});
    });
  };

  const handleSelect = (route: string): void => {
    (async () => {
      await navigate(route);
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to add user sub-page', {error, route});
    });
  };

  const options: AddUserOption[] = [
    {
      route: routes.addCreate(),
      labelKey: 'users:add.type.create.label',
      labelDefault: 'Create User',
      descriptionKey: 'users:add.type.create.description',
      descriptionDefault: 'Create the account now with a password or other credentials.',
      icon: <UserPlus size={28} />,
    },
    {
      route: routes.addInvite(),
      labelKey: 'users:add.type.invite.label',
      labelDefault: 'Invite User',
      descriptionKey: 'users:add.type.invite.description',
      descriptionDefault: 'Send an invite for the user to finish onboarding later.',
      icon: <Send size={28} />,
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
            <AppBreadcrumbs items={[{key: 'add-user', label: t('users:addUser', 'Add User')}]} />
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
              <Stack direction="column" spacing={4} data-testid="add-user-type-select">
                <Typography variant="h1" gutterBottom>
                  {t('users:add.title', 'Add User')}
                </Typography>
                <Typography variant="body1" color="text.secondary">
                  {t(
                    'users:add.subtitle',
                    'Choose whether to create the account now or send an invite for the user to finish onboarding later.',
                  )}
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
