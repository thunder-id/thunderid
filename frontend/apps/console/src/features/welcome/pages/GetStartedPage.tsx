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

import {useConfig} from '@thunderid/contexts';
import {Box, Button, IconButton, Stack, Typography, AppBreadcrumbs} from '@wso2/oxygen-ui';
import {AppWindow, Bot, MCP, SkipForward, X} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useWelcomeClose from '../hooks/useWelcomeClose';

const MotionBox = motion.create(Box);

export default function GetStartedPage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const handleClose = useWelcomeClose();
  const productName = config.brand.product_name;

  const options = [
    {
      id: 'onboard-app',
      icon: <AppWindow size={36} />,
      title: t('common:welcome.getStarted.options.onboardApp.title'),
      description: t('common:welcome.getStarted.options.onboardApp.description', {productName}),
      action: () => void navigate('/welcome/get-started/applications/types'),
      actionLabel: t('common:welcome.getStarted.options.onboardApp.action'),
      disabled: false,
    },
    {
      id: 'secure-ai-agent',
      icon: <Bot size={36} />,
      title: t('common:welcome.getStarted.options.secureAiAgent.title'),
      description: t('common:welcome.getStarted.options.secureAiAgent.description'),
      action: undefined,
      actionLabel: t('common:welcome.getStarted.options.comingSoon'),
      disabled: true,
    },
    {
      id: 'secure-mcp',
      icon: <MCP size={36} />,
      title: t('common:welcome.getStarted.options.secureMcp.title'),
      description: t('common:welcome.getStarted.options.secureMcp.description'),
      action: undefined,
      actionLabel: t('common:welcome.getStarted.options.comingSoon'),
      disabled: true,
    },
  ];

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        <Box
          sx={{
            position: 'sticky',
            top: 0,
            zIndex: 10,
            p: 4,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Stack direction="row" spacing={2} sx={{alignItems: 'center'}}>
            <IconButton
              aria-label={t('common:actions.close')}
              onClick={() => void navigate('/home')}
              sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
            >
              <X size={24} />
            </IconButton>
            <AppBreadcrumbs
              items={[
                {key: 'welcome', label: t('common:welcome.header'), onClick: () => void navigate('/welcome')},
                {
                  key: 'create-project',
                  label: t('common:welcome.createProject.breadcrumb'),
                  onClick: () => void navigate('/welcome/create-project'),
                },
                {key: 'get-started', label: t('common:welcome.getStarted.breadcrumb')},
              ]}
            />
          </Stack>
        </Box>

        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            alignItems: 'center',
            px: {xs: 2, md: 4},
            pb: 8,
          }}
        >
          <MotionBox
            initial={{opacity: 0, y: 20}}
            animate={{opacity: 1, y: 0}}
            transition={{duration: 0.5}}
            sx={{maxWidth: '860px', width: '100%'}}
          >
            <Box sx={{textAlign: 'center', mb: 8}}>
              <Typography
                variant="h1"
                sx={{fontSize: {xs: '1.75rem', sm: '2rem', md: '2.5rem'}, fontWeight: 600, mb: 2}}
              >
                {t('common:welcome.getStarted.title')}
              </Typography>
              <Typography
                variant="body1"
                color="text.secondary"
                sx={{fontSize: {xs: '1rem', sm: '1.125rem'}, maxWidth: '560px', mx: 'auto'}}
              >
                {t('common:welcome.getStarted.subtitle', {productName})}
              </Typography>
            </Box>

            <Box sx={{display: 'flex', flexDirection: {xs: 'column', md: 'row'}, gap: 3, justifyContent: 'center'}}>
              {options.map((option, index) => (
                <MotionBox
                  key={option.id}
                  initial={{opacity: 0, y: 16}}
                  animate={{opacity: 1, y: 0}}
                  transition={{duration: 0.35, delay: 0.2 + index * 0.1}}
                  sx={{flex: '1 1 0', minWidth: 0}}
                >
                  <Box
                    sx={{
                      height: '100%',
                      p: 4,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 2,
                      display: 'flex',
                      flexDirection: 'column',
                      alignItems: 'center',
                      textAlign: 'center',
                      gap: 2,
                      transition: 'all 0.2s',
                      ...(option.disabled ? {opacity: 0.55} : {'&:hover': {boxShadow: 2, borderColor: 'primary.main'}}),
                    }}
                  >
                    <Box
                      sx={{
                        width: 64,
                        height: 64,
                        borderRadius: 3,
                        bgcolor: 'action.selected',
                        color: 'text.secondary',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >
                      {option.icon}
                    </Box>
                    <Typography variant="h6" fontWeight={600}>
                      {option.title}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{flex: 1}}>
                      {option.description}
                    </Typography>
                    <Button
                      variant="contained"
                      onClick={option.action}
                      disabled={option.disabled}
                      sx={{mt: 1, minWidth: 160}}
                    >
                      {option.actionLabel}
                    </Button>
                  </Box>
                </MotionBox>
              ))}
            </Box>

            <MotionBox
              initial={{opacity: 0}}
              animate={{opacity: 1}}
              transition={{duration: 0.4, delay: 0.4}}
              sx={{mt: 4, display: 'flex', justifyContent: 'center'}}
            >
              <Button
                variant="text"
                size="small"
                endIcon={<SkipForward size={14} />}
                onClick={handleClose}
                sx={{color: 'text.secondary'}}
              >
                {t('common:welcome.getStarted.actions.skipToConsole')}
              </Button>
            </MotionBox>
          </MotionBox>
        </Box>
      </Box>
    </Box>
  );
}
