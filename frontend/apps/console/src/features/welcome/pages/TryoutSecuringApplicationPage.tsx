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
import {Box, Button, Stack, Tab, Tabs, Typography, IconButton, LinearProgress, Alert} from '@wso2/oxygen-ui';
import {
  AppWindow,
  Check,
  Copy,
  ExternalLink,
  X,
  BookOpen,
  Eye,
  EyeOff,
  KeyRound,
  LogIn,
  UserCheck,
  UserCircle,
  UserPlus,
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

const WAYFINDER_SAMPLE_URL = 'http://localhost:5173';
const WAYFINDER_MAIL_URL = 'http://localhost:8788';

type ScenarioTab = 'login' | 'signup' | 'profile' | 'recovery' | 'onboard';

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

interface CredentialsBlockProps {
  username: string;
  password: string;
}

function CredentialsBlock({username, password}: CredentialsBlockProps): JSX.Element {
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

interface StepListProps {
  steps: ReactNode[];
  startFrom?: number;
}

function StepList({steps, startFrom = 1}: StepListProps): JSX.Element {
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
      href={WAYFINDER_SAMPLE_URL}
      target="_blank"
      rel="noopener noreferrer"
      style={{color: 'inherit', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: 2}}
    >
      {children}
      <ExternalLink size={12} style={{flexShrink: 0, opacity: 0.7}} />
    </a>
  );
}

function MailLink({children = null}: {children?: ReactNode}): JSX.Element {
  return (
    <a
      href={WAYFINDER_MAIL_URL}
      target="_blank"
      rel="noopener noreferrer"
      style={{color: 'inherit', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: 2}}
    >
      {children}
      <ExternalLink size={12} style={{flexShrink: 0, opacity: 0.7}} />
    </a>
  );
}

function tLink(i18nKey: string, values?: Record<string, unknown>): JSX.Element {
  return <Trans ns="common" i18nKey={i18nKey} values={values} components={{a: <AppLink />, mail: <MailLink />}} />;
}

interface FormField {
  label: string;
  value: string;
}

function FormFieldsBlock({fields}: {fields: FormField[]}): JSX.Element {
  const [copiedLabel, setCopiedLabel] = useState<string | null>(null);

  const handleCopy = (label: string, value: string): void => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopiedLabel(label);
      setTimeout(() => setCopiedLabel(null), 2000);
    });
  };

  return (
    <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 1.5, overflow: 'hidden'}}>
      {fields.map((f, i) => (
        <Box
          key={f.label}
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 2,
            px: 2,
            py: 0.75,
            borderTop: i === 0 ? 'none' : '1px solid',
            borderColor: 'divider',
          }}
        >
          <Typography variant="caption" color="text.secondary" sx={{minWidth: 96, flexShrink: 0}}>
            {f.label}
          </Typography>
          <Typography variant="body2" fontFamily="monospace" sx={{flex: 1}}>
            {f.value}
          </Typography>
          <IconButton
            size="small"
            aria-label={`Copy ${f.label}`}
            onClick={() => handleCopy(f.label, f.value)}
            sx={{color: copiedLabel === f.label ? 'success.main' : 'text.secondary', flexShrink: 0}}
          >
            {copiedLabel === f.label ? <Check size={13} /> : <Copy size={13} />}
          </IconButton>
        </Box>
      ))}
    </Box>
  );
}

export default function TryoutSecuringConsumerApp(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.documentation?.baseUrl ?? '').replace(/\/$/, '');

  const handleClose = useWelcomeClose();
  const [scenarioTab, setScenarioTab] = useState<ScenarioTab>('login');

  const tabDefs: {value: ScenarioTab; label: string; icon: JSX.Element}[] = [
    {value: 'login', label: t('common:welcome.applicationTryout.scenarios.tabs.login'), icon: <LogIn size={15} />},
    {value: 'signup', label: t('common:welcome.applicationTryout.scenarios.tabs.signup'), icon: <UserPlus size={15} />},
    {
      value: 'profile',
      label: t('common:welcome.applicationTryout.scenarios.tabs.profile'),
      icon: <UserCircle size={15} />,
    },
    {
      value: 'recovery',
      label: t('common:welcome.applicationTryout.scenarios.tabs.recovery'),
      icon: <KeyRound size={15} />,
    },
    {
      value: 'onboard',
      label: t('common:welcome.applicationTryout.scenarios.tabs.onboard'),
      icon: <UserCheck size={15} />,
    },
  ];

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={0} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        {/* Header */}
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
                {key: 'tryout', label: t('common:welcome.applicationTryout.breadcrumb')},
              ]}
            />
          </Stack>
        </Box>

        {/* Main Content */}
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
            {/* Title */}
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
                <AppWindow size={13} />
                {t('common:welcome.applicationTryout.overline')}
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
                {t('common:welcome.applicationTryout.subtitle', {productName})}
              </Typography>
            </Box>

            <WayfinderSampleSetup />

            <Typography variant="h3" sx={{fontSize: '1.25rem', fontWeight: 600, mt: 6, mb: 3}}>
              {t('common:welcome.applicationTryout.scenarios.title')}
            </Typography>

            {/* Scenario Tabs */}
            <MotionBox
              initial={{opacity: 0, y: 10}}
              animate={{opacity: 1, y: 0}}
              transition={{duration: 0.4, delay: 0.3}}
              sx={{mt: 3, border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden'}}
            >
              <Tabs
                value={scenarioTab}
                onChange={(_e, v: ScenarioTab) => setScenarioTab(v)}
                variant="scrollable"
                scrollButtons="auto"
                sx={{
                  borderBottom: '1px solid',
                  borderColor: 'divider',
                  minHeight: 44,
                  '& .MuiTab-root': {minHeight: 44, fontSize: '0.8rem', gap: 0.75},
                }}
              >
                {tabDefs.map((tab) => (
                  <Tab key={tab.value} value={tab.value} label={tab.label} icon={tab.icon} iconPosition="start" />
                ))}
              </Tabs>

              <Box sx={{p: 3}}>
                {scenarioTab === 'login' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.login.description')}
                    </Typography>
                    <StepList
                      steps={[
                        tLink('welcome.applicationTryout.scenarios.login.step1'),
                        t('common:welcome.applicationTryout.scenarios.login.step2'),
                      ]}
                    />
                    <CredentialsBlock username="john.doe" password="john.doe" />
                  </Stack>
                )}

                {scenarioTab === 'signup' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.signup.description', {productName})}
                    </Typography>
                    <StepList
                      steps={[
                        tLink('welcome.applicationTryout.scenarios.signup.step1'),
                        t('common:welcome.applicationTryout.scenarios.signup.step2', {productName}),
                        t('common:welcome.applicationTryout.scenarios.signup.step3'),
                      ]}
                    />
                    <FormFieldsBlock
                      fields={[
                        {
                          label: t('common:welcome.applicationTryout.scenarios.signup.sampleFields.username'),
                          value: 'emma.wilson',
                        },
                        {
                          label: t('common:welcome.applicationTryout.scenarios.signup.sampleFields.email'),
                          value: 'emma.wilson@example.com',
                        },
                        {
                          label: t('common:welcome.applicationTryout.scenarios.signup.sampleFields.givenName'),
                          value: 'Emma',
                        },
                        {
                          label: t('common:welcome.applicationTryout.scenarios.signup.sampleFields.familyName'),
                          value: 'Wilson',
                        },
                        {
                          label: t('common:welcome.applicationTryout.scenarios.signup.sampleFields.mobileNumber'),
                          value: '+15550148812',
                        },
                      ]}
                    />
                    <StepList
                      startFrom={4}
                      steps={[
                        t('common:welcome.applicationTryout.scenarios.signup.step4', {productName}),
                        t('common:welcome.applicationTryout.scenarios.signup.step5'),
                      ]}
                    />
                  </Stack>
                )}

                {scenarioTab === 'profile' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.profile.description')}
                    </Typography>
                    <StepList
                      steps={[
                        tLink('welcome.applicationTryout.scenarios.profile.step1'),
                        t('common:welcome.applicationTryout.scenarios.profile.step2'),
                        t('common:welcome.applicationTryout.scenarios.profile.step3', {productName}),
                      ]}
                    />
                    <CredentialsBlock username="john.doe" password="john.doe" />
                  </Stack>
                )}

                {scenarioTab === 'recovery' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.recovery.description')}
                    </Typography>
                    <StepList
                      steps={[
                        tLink('welcome.applicationTryout.scenarios.recovery.step1'),
                        t('common:welcome.applicationTryout.scenarios.recovery.step2', {productName}),
                        t('common:welcome.applicationTryout.scenarios.recovery.step3'),
                        tLink('welcome.applicationTryout.scenarios.recovery.step4', {productName}),
                        t('common:welcome.applicationTryout.scenarios.recovery.step5'),
                        t('common:welcome.applicationTryout.scenarios.recovery.step6'),
                      ]}
                    />
                  </Stack>
                )}

                {scenarioTab === 'onboard' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.onboard.description', {productName})}
                    </Typography>
                    <Alert severity="info" sx={{fontSize: '0.8rem'}}>
                      {t('common:welcome.applicationTryout.scenarios.onboard.smtpNote', {productName})}
                    </Alert>
                    <StepList
                      steps={[
                        t('common:welcome.applicationTryout.scenarios.onboard.step1', {productName}),
                        t('common:welcome.applicationTryout.scenarios.onboard.step2'),
                        t('common:welcome.applicationTryout.scenarios.onboard.step3'),
                        t('common:welcome.applicationTryout.scenarios.onboard.step4'),
                        tLink('welcome.applicationTryout.scenarios.onboard.step5'),
                        t('common:welcome.applicationTryout.scenarios.onboard.step6'),
                        t('common:welcome.applicationTryout.scenarios.onboard.step7'),
                      ]}
                    />
                  </Stack>
                )}
              </Box>
            </MotionBox>

            {/* Read docs link */}
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
                sx={{px: 2}}
                onClick={() => {
                  window.open(`${docsBaseUrl}/use-cases/b2c/try-it-out`, '_blank', 'noopener,noreferrer');
                }}
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
