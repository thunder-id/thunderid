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
import {Box, Button, Stack, Tab, Tabs, Typography, IconButton, LinearProgress, AppBreadcrumbs} from '@wso2/oxygen-ui';
import {BookOpen, Bot, CalendarCheck, Check, Copy, Search, Shield, X} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import {type JSX, useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import AIAgentApiKeySetup from '../components/AIAgentApiKeySetup';
import CodeInline from '../components/CodeInline';
import CredentialsBlock from '../components/CredentialsBlock';
import ExternalLink from '../components/ExternalLink';
import StepList from '../components/StepList';
import WayfinderSampleSetup from '../components/WayfinderSampleSetup';
import {WAYFINDER_APP_URL} from '../constants/sample-urls';
import useWelcomeClose from '../hooks/useWelcomeClose';

const MotionBox = motion.create(Box);

type ScenarioTab = 'protect' | 'browse' | 'book';

function ChatPromptBlock({text}: {text: string}): JSX.Element {
  const [copied, setCopied] = useState(false);

  const handleCopy = (): void => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        px: 2,
        py: 1.5,
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        bgcolor: 'action.hover',
      }}
    >
      <Typography variant="body2" fontFamily="monospace" sx={{flex: 1, fontStyle: 'italic'}}>
        {`"${text}"`}
      </Typography>
      <IconButton
        size="small"
        aria-label="Copy prompt"
        onClick={handleCopy}
        sx={{color: copied ? 'success.main' : 'text.secondary', flexShrink: 0}}
      >
        {copied ? <Check size={14} /> : <Copy size={14} />}
      </IconButton>
    </Box>
  );
}

export default function TryoutSecuringAIAgentsPage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const handleClose = useWelcomeClose();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.documentation?.baseUrl ?? '').replace(/\/$/, '');
  const releasesUrl = config.brand.documentation?.releasesUrl ?? '';

  const [scenarioTab, setScenarioTab] = useState<ScenarioTab>('protect');

  const tabDefs: {value: ScenarioTab; label: string; icon: JSX.Element}[] = [
    {value: 'protect', label: t('common:welcome.aiAgentsTryout.scenarios.tabs.protect'), icon: <Shield size={15} />},
    {value: 'browse', label: t('common:welcome.aiAgentsTryout.scenarios.tabs.browse'), icon: <Search size={15} />},
    {value: 'book', label: t('common:welcome.aiAgentsTryout.scenarios.tabs.book'), icon: <CalendarCheck size={15} />},
  ];

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={0} sx={{height: 6}} />
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
              onClick={handleClose}
              sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
            >
              <X size={24} />
            </IconButton>
            <AppBreadcrumbs
              items={[
                {
                  key: 'welcome',
                  label: t('common:welcome.header'),
                  onClick: () => void navigate(RouteConfig.welcome.root()),
                },
                {key: 'tryout', label: t('common:welcome.aiAgentsTryout.breadcrumb')},
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
            <Box sx={{textAlign: 'center', mb: 6}}>
              <Typography
                variant="overline"
                color="text.secondary"
                sx={{
                  letterSpacing: 2,
                  mb: 1,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 0.75,
                }}
              >
                <Bot size={13} />
                {t('common:welcome.aiAgentsTryout.overline')}
              </Typography>
              <Typography
                variant="h1"
                sx={{fontSize: {xs: '1.75rem', sm: '2rem', md: '2.5rem'}, fontWeight: 600, mb: 2}}
              >
                {t('common:welcome.tryout.title')}
              </Typography>
              <Typography
                variant="body1"
                color="text.secondary"
                sx={{fontSize: {xs: '1rem', sm: '1.125rem'}, maxWidth: '580px', mx: 'auto'}}
              >
                {t('common:welcome.aiAgentsTryout.subtitle', {productName})}
              </Typography>
            </Box>

            <WayfinderSampleSetup />

            <Box sx={{mt: 3}}>
              <AIAgentApiKeySetup releasesUrl={releasesUrl} />
            </Box>

            <MotionBox
              initial={{opacity: 0, y: 10}}
              animate={{opacity: 1, y: 0}}
              transition={{duration: 0.4, delay: 0.3}}
              sx={{mt: 3}}
            >
              <Typography variant="h3" sx={{fontSize: '1.25rem', fontWeight: 600, mt: 6, mb: 3}}>
                {t('common:welcome.aiAgentsTryout.scenarios.title')}
              </Typography>

              <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden'}}>
                <Tabs
                  value={scenarioTab}
                  onChange={(_e, v: ScenarioTab) => setScenarioTab(v)}
                  variant="scrollable"
                  scrollButtons="auto"
                  sx={{
                    borderBottom: '1px solid',
                    borderColor: 'divider',
                    bgcolor: 'action.selected',
                    minHeight: 44,
                    '& .MuiTab-root': {minHeight: 44, fontSize: '0.8rem', gap: 0.75},
                  }}
                >
                  {tabDefs.map((tab) => (
                    <Tab key={tab.value} value={tab.value} label={tab.label} icon={tab.icon} iconPosition="start" />
                  ))}
                </Tabs>

                <Box sx={{p: 3}}>
                  {scenarioTab === 'protect' && (
                    <Stack spacing={2}>
                      <Typography variant="body2" color="text.secondary">
                        {t('common:welcome.aiAgentsTryout.scenarios.protect.description')}
                      </Typography>
                      <StepList
                        steps={[
                          <Trans
                            key="step1"
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.protect.step1"
                            components={{a: <ExternalLink href={WAYFINDER_APP_URL} />}}
                          />,
                        ]}
                      />
                      <Stack spacing={1}>
                        <Typography
                          variant="caption"
                          color="success.main"
                          sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5}}
                        >
                          <Check size={12} />
                          <Trans
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.protect.johnLabel"
                            components={{code: <CodeInline />}}
                          />
                        </Typography>
                        <CredentialsBlock username="john.doe" password="john.doe" />
                      </Stack>
                      <StepList
                        startFrom={2}
                        steps={[
                          <Trans
                            key="step2"
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.protect.step2"
                            components={{code: <CodeInline />}}
                          />,
                          t('common:welcome.aiAgentsTryout.scenarios.protect.step3'),
                        ]}
                      />
                      <Stack spacing={1}>
                        <Typography
                          variant="caption"
                          color="error.main"
                          sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5}}
                        >
                          <X size={12} />
                          <Trans
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.protect.janeLabel"
                            components={{code: <CodeInline />}}
                          />
                        </Typography>
                        <CredentialsBlock username="jane.smith" password="jane.smith" />
                      </Stack>
                      <StepList startFrom={4} steps={[t('common:welcome.aiAgentsTryout.scenarios.protect.step4')]} />
                    </Stack>
                  )}

                  {scenarioTab === 'browse' && (
                    <Stack spacing={2}>
                      <Typography variant="body2" color="text.secondary">
                        {t('common:welcome.aiAgentsTryout.scenarios.browse.description')}
                      </Typography>
                      <StepList
                        steps={[
                          <Trans
                            key="step1"
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.browse.step1"
                            components={{a: <ExternalLink href={WAYFINDER_APP_URL} />}}
                          />,
                        ]}
                      />
                      <CredentialsBlock username="john.doe" password="john.doe" />
                      <StepList steps={[t('common:welcome.aiAgentsTryout.scenarios.browse.step2')]} startFrom={2} />
                      <ChatPromptBlock text="What flights are there from Colombo to Singapore?" />
                      <StepList
                        startFrom={3}
                        steps={[
                          t('common:welcome.aiAgentsTryout.scenarios.browse.step3'),
                          <Trans
                            key="step4"
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.browse.step4"
                            components={{code: <CodeInline />}}
                          />,
                        ]}
                      />
                      <ChatPromptBlock text={t('common:welcome.aiAgentsTryout.scenarios.browse.step4Prompt')} />
                    </Stack>
                  )}

                  {scenarioTab === 'book' && (
                    <Stack spacing={2}>
                      <Typography variant="body2" color="text.secondary">
                        {t('common:welcome.aiAgentsTryout.scenarios.book.description')}
                      </Typography>
                      <StepList
                        steps={[
                          <Trans
                            key="step1"
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.book.step1"
                            components={{a: <ExternalLink href={WAYFINDER_APP_URL} />}}
                          />,
                        ]}
                      />
                      <CredentialsBlock username="john.doe" password="john.doe" />
                      <StepList startFrom={2} steps={[t('common:welcome.aiAgentsTryout.scenarios.book.step2')]} />
                      <ChatPromptBlock text={t('common:welcome.aiAgentsTryout.scenarios.book.step2Prompt')} />
                      <StepList
                        startFrom={3}
                        steps={[
                          <Trans
                            key="step3"
                            ns="common"
                            i18nKey="welcome.aiAgentsTryout.scenarios.book.step3"
                            components={{code: <CodeInline />}}
                          />,
                          t('common:welcome.aiAgentsTryout.scenarios.book.step4'),
                        ]}
                      />
                    </Stack>
                  )}
                </Box>
              </Box>
            </MotionBox>

            <MotionBox
              initial={{opacity: 0}}
              animate={{opacity: 1}}
              transition={{duration: 0.4, delay: 0.5}}
              sx={{mt: 4, display: 'flex', justifyContent: 'center'}}
            >
              <Button
                variant="text"
                size="small"
                startIcon={<BookOpen size={16} />}
                onClick={() =>
                  window.open(`${docsBaseUrl}/use-cases/ai-agents/try-it-out`, '_blank', 'noopener,noreferrer')
                }
              >
                {t('common:welcome.tryout.actions.readDocs')}
              </Button>
            </MotionBox>
          </MotionBox>
        </Box>
      </Box>
    </Box>
  );
}
