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
  Rocket,
  MCP,
} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import getWelcomeDismissedStorageKey from '../utils/getWelcomeDismissedStorageKey';

const MotionBox = motion.create(Box);

export default function WelcomePage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const theme = useTheme();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.docs_url ?? '').replace(/\/$/, '');

  const handleClose = (): void => {
    sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
    void navigate('/home');
  };

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
      id: 'learn-b2c',
      icon: <Users size={18} />,
      label: t('common:welcome.tryoutProduct.b2c'),
      description: t('common:welcome.tryoutProduct.b2cDesc'),
      url: `${docsBaseUrl}/use-cases/b2c/try-it-out`,
    },
    {
      id: 'learn-ai-agents',
      icon: <Bot size={18} />,
      label: t('common:welcome.tryoutProduct.aiAgents'),
      description: t('common:welcome.tryoutProduct.aiAgentsDesc'),
      url: `${docsBaseUrl}/use-cases/ai-agents/try-it-out`,
    },
    {
      id: 'learn-mcp',
      icon: <MCP size={18} />,
      label: t('common:welcome.tryoutProduct.mcp'),
      description: t('common:welcome.tryoutProduct.mcpDesc'),
      url: `${docsBaseUrl}/use-cases/ai-agents/mcp-authorization/try-it-out`,
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
          {/* Two Column Layout for Start and Walkthrough */}
          <Box
            sx={{
              display: 'flex',
              flexDirection: {xs: 'column', md: 'row'},
              gap: {xs: 3, md: 10},
            }}
          >
            {/* Left Column - Start */}
            <Box
              sx={{
                flex: 1,
              }}
            >
              <Stack spacing={2}>
                {startActions.map((action, index) => (
                  <MotionBox
                    key={action.id}
                    initial={{opacity: 0, x: -10}}
                    animate={{opacity: 1, x: 0}}
                    transition={{duration: 0.2, delay: 0.15 + index * 0.05}}
                  >
                    <Card
                      variant="outlined"
                      sx={{
                        p: 2.5,
                        cursor: 'pointer',
                        transition: 'all 0.2s',
                        '&:hover': {
                          transform: 'translateY(-2px)',
                          boxShadow: 2,
                          borderColor: 'primary.main',
                        },
                      }}
                      onClick={action.action}
                    >
                      <Stack spacing={1.5}>
                        <Box sx={{display: 'flex', alignItems: 'flex-start', gap: 1.5}}>
                          <Box
                            sx={{
                              width: 40,
                              height: 40,
                              borderRadius: 1,
                              bgcolor: 'primary.lighter',
                              color: 'primary.main',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              flexShrink: 0,
                            }}
                          >
                            {action.icon}
                          </Box>
                          <Box sx={{flex: 1}}>
                            <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5}}>
                              <Typography variant="subtitle1" fontWeight={600}>
                                {action.label}
                              </Typography>
                              <ChevronRight size={18} />
                            </Box>
                            <Typography variant="body2" color="text.secondary">
                              {action.description}
                            </Typography>
                          </Box>
                        </Box>
                      </Stack>
                    </Card>
                  </MotionBox>
                ))}
              </Stack>
            </Box>

            {/* Right Column - Walkthrough */}
            <Box
              sx={{
                flex: 1,
                display: {xs: 'none', md: 'block'},
              }}
            >
              <MotionBox
                initial={{opacity: 0, y: 20}}
                animate={{opacity: 1, y: 0}}
                transition={{duration: 0.3, delay: 0.3}}
              >
                <Box
                  sx={{
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: 1,
                  }}
                >
                  <Typography
                    variant="h2"
                    sx={{
                      py: 2.8,
                      px: 2.2,
                      m: 0,
                      backgroundColor: 'background.paper',
                      fontSize: '1.25rem',
                      fontWeight: 600,
                      borderRadius: '8px 8px 0 0',
                    }}
                  >
                    <Rocket size={20} style={{marginRight: 12, marginBottom: 2, verticalAlign: 'middle'}} />
                    {t('common:welcome.sections.tryoutProduct', {productName})}
                  </Typography>
                  <Box
                    sx={{
                      overflow: 'hidden',
                    }}
                  >
                    {learnProduct.map((item, index) => (
                      <MotionBox
                        key={item.id}
                        initial={{opacity: 0, x: 10}}
                        animate={{opacity: 1, x: 0}}
                        transition={{duration: 0.2, delay: 0.3 + index * 0.05}}
                      >
                        <Box
                          component="a"
                          href={item.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 1.5,
                            cursor: 'pointer',
                            color: 'inherit',
                            textDecoration: 'none',
                            px: 2,
                            py: 1.5,
                            borderBottom: index < learnProduct.length - 1 ? '1px solid' : 'none',
                            borderColor: 'divider',
                            '&:hover': {
                              bgcolor: 'action.hover',
                              '& .learnproduct-title': {
                                color: 'primary.main',
                                textDecoration: 'underline',
                              },
                            },
                          }}
                        >
                          <Box sx={{color: 'text.secondary', display: 'flex', flexShrink: 0, mr: 0.5}}>{item.icon}</Box>
                          <Stack spacing={0.5} sx={{flex: 1}}>
                            <Typography
                              className="learnproduct-title"
                              variant="body1"
                              fontWeight={500}
                              sx={{transition: 'all 0.2s'}}
                            >
                              {item.label}
                            </Typography>
                            <Typography variant="body2" color="text.secondary">
                              {item.description}
                            </Typography>
                          </Stack>
                          <Box sx={{color: 'text.disabled', display: 'flex', flexShrink: 0}}>
                            <ExternalLink size={14} />
                          </Box>
                        </Box>
                      </MotionBox>
                    ))}
                  </Box>
                </Box>

                <Stack spacing={2} sx={{mt: 4}}>
                  {walkthroughs.map((walkthrough, index) => (
                    <MotionBox
                      key={walkthrough.id}
                      initial={{opacity: 0, x: 10}}
                      animate={{opacity: 1, x: 0}}
                      transition={{duration: 0.2, delay: 0.35 + index * 0.05}}
                      sx={{px: 2}}
                    >
                      <Box
                        onClick={walkthrough.action}
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 1.5,
                          cursor: 'pointer',
                          py: 1,
                          '&:hover': {
                            '& .walkthrough-title': {
                              color: 'primary.main',
                              textDecoration: 'underline',
                            },
                          },
                        }}
                      >
                        <Box sx={{color: 'text.secondary', display: 'flex', flexShrink: 0}}>{walkthrough.icon}</Box>
                        <Stack spacing={0.5}>
                          <Typography
                            className="walkthrough-title"
                            variant="body1"
                            fontWeight={500}
                            sx={{
                              transition: 'all 0.2s',
                            }}
                          >
                            {walkthrough.label}
                            <Box sx={{color: 'text.disabled', display: 'inline-block', ml: 1}}>
                              <ExternalLink size={14} />
                            </Box>
                          </Typography>
                          <Typography variant="body2" color="text.secondary">
                            {walkthrough.description}
                          </Typography>
                        </Stack>
                      </Box>
                    </MotionBox>
                  ))}
                </Stack>
              </MotionBox>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
