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
import {Alert, Box, Button, Stack, Tab, Tabs, Typography, IconButton, LinearProgress} from '@wso2/oxygen-ui';
import {
  BookOpen,
  Bot,
  CalendarCheck,
  Check,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  Search,
  Shield,
  X,
} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX, ReactNode} from 'react';
import {useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import WayfinderSampleSetup from '../components/WayfinderSampleSetup';
import useWelcomeClose from '../hooks/useWelcomeClose';
import AppBreadcrumbs from '@/components/AppBreadcrumbs';

const MotionBox = motion.create(Box);

type ScenarioTab = 'protect' | 'browse' | 'book';

interface CredentialRowProps {
  field: 'username' | 'password';
  value: string;
  showPassword: boolean;
  isCopied: boolean;
  onToggleShow: () => void;
  onCopy: () => void;
}

function CredentialRow({field, value, showPassword, isCopied, onToggleShow, onCopy}: CredentialRowProps): JSX.Element {
  const isPassword = field === 'password';
  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1.5,
        px: 2,
        py: 1,
        display: 'flex',
        alignItems: 'center',
        gap: 1,
      }}
    >
      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography variant="caption" color="text.secondary" sx={{display: 'block', textTransform: 'capitalize'}}>
          {field}
        </Typography>
        <Typography variant="body2" fontFamily="monospace">
          {isPassword && !showPassword ? '••••••••' : value}
        </Typography>
      </Box>
      {isPassword && (
        <IconButton
          size="small"
          aria-label={showPassword ? 'Hide password' : 'Show password'}
          onClick={onToggleShow}
          sx={{color: 'text.secondary'}}
        >
          {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
        </IconButton>
      )}
      <IconButton
        size="small"
        aria-label={`Copy ${field}`}
        onClick={onCopy}
        sx={{color: isCopied ? 'success.main' : 'text.secondary'}}
      >
        {isCopied ? <Check size={14} /> : <Copy size={14} />}
      </IconButton>
    </Box>
  );
}

function CredentialsBlock({username, password}: {username: string; password: string}): JSX.Element {
  const [showPassword, setShowPassword] = useState(false);
  const [copiedField, setCopiedField] = useState<'username' | 'password' | null>(null);

  const handleCopy = (field: 'username' | 'password', value: string): void => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopiedField(field);
      setTimeout(() => setCopiedField(null), 2000);
    });
  };

  return (
    <Stack spacing={1}>
      <CredentialRow
        field="username"
        value={username}
        showPassword={showPassword}
        isCopied={copiedField === 'username'}
        onToggleShow={() => setShowPassword((v) => !v)}
        onCopy={() => handleCopy('username', username)}
      />
      <CredentialRow
        field="password"
        value={password}
        showPassword={showPassword}
        isCopied={copiedField === 'password'}
        onToggleShow={() => setShowPassword((v) => !v)}
        onCopy={() => handleCopy('password', password)}
      />
    </Stack>
  );
}

function StepList({steps, startFrom = 1}: {steps: ReactNode[]; startFrom?: number}): JSX.Element {
  return (
    <Stack spacing={1} component="ol" sx={{pl: 0, m: 0, listStyle: 'none'}}>
      {steps.map((step, i) => (
        <Stack
          // eslint-disable-next-line react/no-array-index-key
          key={i}
          component="li"
          direction="row"
          spacing={1.5}
          alignItems="flex-start"
        >
          <Box
            sx={{
              width: 20,
              height: 20,
              borderRadius: '50%',
              bgcolor: 'action.selected',
              color: 'text.secondary',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '0.7rem',
              fontWeight: 700,
              flexShrink: 0,
              mt: 0.15,
            }}
          >
            {startFrom + i}
          </Box>
          <Typography variant="body2" color="text.secondary" sx={{flex: 1}}>
            {step}
          </Typography>
        </Stack>
      ))}
    </Stack>
  );
}

function AppLink({children = null}: {children?: ReactNode}): JSX.Element {
  return (
    <a
      href="http://localhost:5173"
      target="_blank"
      rel="noopener noreferrer"
      style={{color: 'inherit', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: 2}}
    >
      {children}
      <ExternalLink size={12} style={{flexShrink: 0, opacity: 0.7}} />
    </a>
  );
}

function tLink(i18nKey: string): JSX.Element {
  return <Trans ns="common" i18nKey={i18nKey} components={{a: <AppLink />}} />;
}

export default function TryoutSecuringAIAgentsPage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const handleClose = useWelcomeClose();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.documentation?.baseUrl ?? '').replace(/\/$/, '');

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
                {key: 'welcome', label: t('common:welcome.header'), onClick: () => void navigate('/welcome')},
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

            <MotionBox
              initial={{opacity: 0, y: 10}}
              animate={{opacity: 1, y: 0}}
              transition={{duration: 0.4, delay: 0.3}}
              sx={{mt: 3}}
            >
              <Typography variant="h3" sx={{fontSize: '1.25rem', fontWeight: 600, mb: 2}}>
                {t('common:welcome.aiAgentsTryout.scenarios.title')}
              </Typography>
              <Alert severity="info" icon={<Bot size={16} />} sx={{mb: 2, fontSize: '0.8rem'}}>
                {t('common:welcome.aiAgentsTryout.scenarios.apiKeyNote')}
              </Alert>

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
                          tLink('welcome.aiAgentsTryout.scenarios.protect.step1'),
                          t('common:welcome.aiAgentsTryout.scenarios.protect.step2'),
                          t('common:welcome.aiAgentsTryout.scenarios.protect.step3'),
                          t('common:welcome.aiAgentsTryout.scenarios.protect.step4'),
                        ]}
                      />
                      <Stack spacing={1}>
                        <Typography variant="caption" color="text.secondary">
                          {t('common:welcome.aiAgentsTryout.scenarios.protect.johnLabel')}
                        </Typography>
                        <CredentialsBlock username="john.doe" password="john.doe" />
                      </Stack>
                      <Stack spacing={1}>
                        <Typography variant="caption" color="text.secondary">
                          {t('common:welcome.aiAgentsTryout.scenarios.protect.janeLabel')}
                        </Typography>
                        <CredentialsBlock username="jane.smith" password="jane.smith" />
                      </Stack>
                    </Stack>
                  )}

                  {scenarioTab === 'browse' && (
                    <Stack spacing={2}>
                      <Typography variant="body2" color="text.secondary">
                        {t('common:welcome.aiAgentsTryout.scenarios.browse.description')}
                      </Typography>
                      <StepList
                        steps={[
                          tLink('welcome.aiAgentsTryout.scenarios.browse.step1'),
                          t('common:welcome.aiAgentsTryout.scenarios.browse.step2'),
                          t('common:welcome.aiAgentsTryout.scenarios.browse.step3'),
                          t('common:welcome.aiAgentsTryout.scenarios.browse.step4'),
                        ]}
                      />
                      <CredentialsBlock username="john.doe" password="john.doe" />
                    </Stack>
                  )}

                  {scenarioTab === 'book' && (
                    <Stack spacing={2}>
                      <Typography variant="body2" color="text.secondary">
                        {t('common:welcome.aiAgentsTryout.scenarios.book.description')}
                      </Typography>
                      <StepList
                        steps={[
                          tLink('welcome.aiAgentsTryout.scenarios.book.step1'),
                          t('common:welcome.aiAgentsTryout.scenarios.book.step2'),
                          t('common:welcome.aiAgentsTryout.scenarios.book.step3'),
                          t('common:welcome.aiAgentsTryout.scenarios.book.step4'),
                          t('common:welcome.aiAgentsTryout.scenarios.book.step5'),
                        ]}
                      />
                      <CredentialsBlock username="john.doe" password="john.doe" />
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
