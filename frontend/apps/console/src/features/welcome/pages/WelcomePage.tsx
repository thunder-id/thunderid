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
import {Box, Card, IconButton, Stack, Typography, useTheme} from '@wso2/oxygen-ui';
import {
  FolderOpen,
  X,
  ChevronRight,
  PackagePlus,
  Bot,
  Users,
  ExternalLink,
  Lightbulb,
  MCP,
} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useWelcomeClose from '../hooks/useWelcomeClose';
import getWelcomeDismissedStorageKey from '../utils/getWelcomeDismissedStorageKey';

const MotionBox = motion.create(Box);

export default function WelcomePage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const theme = useTheme();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.documentation?.baseUrl ?? '').replace(/\/$/, '');
  const handleClose = useWelcomeClose();

  const handleCreateNewProject = (): void => {
    sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
    void navigate('/welcome/create-project');
  };

  const startActions = [
    {
      id: 'new-project',
      icon: <PackagePlus size={20} />,
      label: t('common:welcome.start.newProject'),
      description: t('common:welcome.start.newProjectDesc'),
      action: handleCreateNewProject,
    },
    {
      id: 'open-import',
      icon: <FolderOpen size={20} />,
      label: t('common:welcome.start.openImport'),
      description: t('common:welcome.start.openImportDesc', {productName}),
      action: () => {
        sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
        void navigate('/welcome/open-project');
      },
    },
  ];

  const learnProduct = [
    {
      id: 'learn-securing-application',
      icon: <Users size={18} />,
      label: t('common:welcome.tryoutProduct.securingApplication'),
      description: t('common:welcome.tryoutProduct.securingApplicationDesc'),
      action: () => {
        sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
        void navigate('/welcome/tryout/securing-application');
      },
    },
    {
      id: 'learn-ai-agents',
      icon: <Bot size={18} />,
      label: t('common:welcome.tryoutProduct.aiAgents'),
      description: t('common:welcome.tryoutProduct.aiAgentsDesc'),
      action: () => window.open(`${docsBaseUrl}/use-cases/ai-agents/try-it-out`, '_blank', 'noopener,noreferrer'),
      endIcon: <ExternalLink size={14} />,
    },
    {
      id: 'learn-mcp',
      icon: <MCP size={18} />,
      label: t('common:welcome.tryoutProduct.mcp'),
      description: t('common:welcome.tryoutProduct.mcpDesc'),
      action: () =>
        window.open(`${docsBaseUrl}/use-cases/ai-agents/mcp-authorization/try-it-out`, '_blank', 'noopener,noreferrer'),
      endIcon: <ExternalLink size={14} />,
    },
  ];

  const walkthroughs = [
    {
      id: 'learn-fundamentals',
      icon: <Lightbulb size={18} />,
      label: t('common:welcome.walkthrough.learnFundamentals'),
      description: t('common:welcome.walkthrough.learnFundamentalsDesc'),
      action: () => window.open(`${docsBaseUrl}`, '_blank', 'noopener,noreferrer'),
    },
  ];

  return (
    <Box
      sx={{
        height: '100vh',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* Header with close button */}
      <Box sx={{p: 4, display: 'flex', justifyContent: 'flex-start', alignItems: 'center'}}>
        <IconButton
          aria-label={t('common:actions.close')}
          onClick={handleClose}
          sx={{
            bgcolor: 'background.paper',
            '&:hover': {bgcolor: 'action.hover'},
            boxShadow: 1,
          }}
        >
          <X size={24} />
        </IconButton>
      </Box>

      {/* Main Content - Two Column Layout */}
      <Box
        sx={{
          flex: 1,
          overflow: 'auto',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          mt: {xs: 0, md: -16},
          px: {xs: 2, md: 3},
          py: {xs: 3, md: 3},
        }}
      >
        {/* Hero Title */}
        <Box
          sx={{
            textAlign: 'center',
            mb: {xs: 4, md: 6},
            pb: 2,
          }}
        >
          <MotionBox initial={{opacity: 0, y: 20}} animate={{opacity: 1, y: 0}} transition={{duration: 0.4}}>
            <Typography
              variant="h2"
              sx={{
                fontSize: {xs: '1.75rem', sm: '2rem', md: '2.25rem'},
                fontWeight: 600,
                mb: 1,
              }}
            >
              {'👋 '}
              {t('common:welcome.hero.titlePrefix')}{' '}
              <Box
                component="span"
                sx={{
                  background: theme.gradient?.primary ?? 'linear-gradient(90deg, #10996b 0%, #13b57d 100%)',
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                  backgroundClip: 'text',
                }}
              >
                {productName}
              </Box>
            </Typography>
            <Typography
              variant="body1"
              color="text.secondary"
              sx={{
                fontSize: {xs: '0.95rem', sm: '1rem'},
              }}
            >
              {t('common:welcome.hero.subtitle')}
            </Typography>
          </MotionBox>
        </Box>

        {/* Content Container with max width */}
        <Box
          sx={{
            maxWidth: '1000px',
            width: '100%',
            margin: '0 auto',
            display: 'flex',
            flexDirection: 'column',
            gap: {xs: 3, md: 4},
          }}
        >
          <Box
            sx={{
              display: 'flex',
              flexDirection: {xs: 'column', md: 'row'},
              gap: {xs: 3, md: 6},
              alignItems: 'stretch',
            }}
          >
            {/* Left Column - Start */}
            <Box sx={{flex: 1, display: 'flex', flexDirection: 'column', gap: 2}}>
              <Typography
                variant="overline"
                color="text.secondary"
                sx={{letterSpacing: 1.5, display: 'block', mb: 0.5}}
              >
                {t('common:welcome.sections.start')}
              </Typography>
              {startActions.map((action, index) => (
                <MotionBox
                  key={action.id}
                  initial={{opacity: 0, y: 10}}
                  animate={{opacity: 1, y: 0}}
                  transition={{duration: 0.25, delay: 0.1 + index * 0.07}}
                >
                  <Card
                    variant="outlined"
                    onClick={action.action}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        action.action();
                      }
                    }}
                    role="button"
                    tabIndex={0}
                    sx={{
                      p: 3,
                      cursor: 'pointer',
                      transition: 'all 0.2s',
                      border: '1px solid',
                      borderColor: 'divider',
                      '&:hover': {
                        transform: 'translateY(-2px)',
                        boxShadow: 3,
                        borderColor: 'primary.main',
                        '& .start-label': {color: 'primary.main'},
                      },
                    }}
                  >
                    <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
                      <Box
                        sx={{
                          width: 44,
                          height: 44,
                          borderRadius: 1.5,
                          bgcolor: 'primary.main',
                          color: '#fff',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          flexShrink: 0,
                        }}
                      >
                        {action.icon}
                      </Box>
                      <Box sx={{flex: 1, minWidth: 0}}>
                        <Typography
                          className="start-label"
                          variant="subtitle1"
                          fontWeight={600}
                          sx={{transition: 'color 0.2s', mb: 0.25}}
                        >
                          {action.label}
                        </Typography>
                        <Typography variant="body2" color="text.secondary" noWrap>
                          {action.description}
                        </Typography>
                      </Box>
                      <ChevronRight size={18} style={{flexShrink: 0, opacity: 0.4}} />
                    </Box>
                  </Card>
                </MotionBox>
              ))}

              <MotionBox
                initial={{opacity: 0}}
                animate={{opacity: 1}}
                transition={{duration: 0.3, delay: 0.35}}
                sx={{mt: 1}}
              >
                {walkthroughs.map((walkthrough) => (
                  <Box
                    key={walkthrough.id}
                    onClick={walkthrough.action}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        walkthrough.action();
                      }
                    }}
                    role="button"
                    tabIndex={0}
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1.5,
                      cursor: 'pointer',
                      px: 1,
                      py: 1,
                      borderRadius: 1,
                      '&:hover': {'& .walkthrough-title': {color: 'primary.main', textDecoration: 'underline'}},
                    }}
                  >
                    <Box sx={{color: 'text.secondary', display: 'flex', flexShrink: 0}}>{walkthrough.icon}</Box>
                    <Typography
                      className="walkthrough-title"
                      variant="body2"
                      color="text.secondary"
                      sx={{transition: 'all 0.2s', flex: 1, display: 'flex', alignItems: 'center', gap: 0.5}}
                    >
                      {walkthrough.label}
                      <ExternalLink size={12} style={{flexShrink: 0, opacity: 0.4}} />
                    </Typography>
                  </Box>
                ))}
              </MotionBox>
            </Box>

            {/* Divider */}
            <Box sx={{width: '1px', bgcolor: 'divider', display: {xs: 'none', md: 'block'}, alignSelf: 'stretch'}} />

            {/* Right Column - Tryout scenarios */}
            <Box sx={{flex: 1, display: 'flex', flexDirection: 'column', gap: 2}}>
              <Typography
                variant="overline"
                color="text.secondary"
                sx={{letterSpacing: 1.5, display: 'block', mb: 0.5}}
              >
                {t('common:welcome.sections.tryoutProduct', {productName})}
              </Typography>
              <MotionBox
                initial={{opacity: 0, y: 10}}
                animate={{opacity: 1, y: 0}}
                transition={{duration: 0.3, delay: 0.2}}
                sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden'}}
              >
                {learnProduct.map((item, index) => (
                  <Box
                    key={item.id}
                    onClick={item.action}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        item.action();
                      }
                    }}
                    role="button"
                    tabIndex={0}
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 2,
                      cursor: 'pointer',
                      px: 2.5,
                      py: 2,
                      borderBottom: index < learnProduct.length - 1 ? '1px solid' : 'none',
                      borderColor: 'divider',
                      transition: 'background 0.15s',
                      '&:hover': {
                        bgcolor: 'action.hover',
                        '& .tryout-label': {color: 'primary.main'},
                        '& .tryout-end-icon': {opacity: 1},
                      },
                    }}
                  >
                    <Box
                      sx={{
                        width: 36,
                        height: 36,
                        borderRadius: 1.5,
                        bgcolor: 'action.selected',
                        color: 'text.secondary',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        flexShrink: 0,
                      }}
                    >
                      {item.icon}
                    </Box>
                    <Stack spacing={0.25} sx={{flex: 1, minWidth: 0}}>
                      <Typography
                        className="tryout-label"
                        variant="body2"
                        fontWeight={600}
                        sx={{transition: 'color 0.2s'}}
                      >
                        {item.label}
                      </Typography>
                      <Typography variant="caption" color="text.secondary" noWrap>
                        {item.description}
                      </Typography>
                    </Stack>
                    <Box
                      className="tryout-end-icon"
                      sx={{
                        color: 'text.disabled',
                        display: 'flex',
                        flexShrink: 0,
                        opacity: 0.5,
                        transition: 'opacity 0.2s',
                      }}
                    >
                      {item.endIcon ?? <ChevronRight size={14} />}
                    </Box>
                  </Box>
                ))}
              </MotionBox>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
