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
import {
  Box,
  Button,
  Chip,
  Divider,
  Stack,
  Tab,
  Tabs,
  Typography,
  IconButton,
  LinearProgress,
  Alert,
  AppBreadcrumbs,
} from '@wso2/oxygen-ui';
import {
  AppWindow,
  X,
  BookOpen,
  KeyRound,
  LogIn,
  Share2,
  ShieldCheck,
  UserCheck,
  UserPlus,
} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import {useState, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import CodeInline from '../components/CodeInline';
import CredentialsBlock from '../components/CredentialsBlock';
import ExternalLink from '../components/ExternalLink';
import FormFieldsBlock from '../components/FormFieldsBlock';
import StepList from '../components/StepList';
import WayfinderSampleSetup from '../components/WayfinderSampleSetup';
import {WAYFINDER_APP_URL, WAYFINDER_MAIL_URL} from '../constants/sample-urls';
import useWelcomeClose from '../hooks/useWelcomeClose';

const MotionBox = motion.create(Box);

type ScenarioTab = 'login' | 'signup' | 'recovery' | 'onboard' | 'mfa' | 'social';

export default function TryoutSecuringConsumerApp(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.documentation?.baseUrl ?? '').replace(/\/$/, '');

  const handleClose = useWelcomeClose();
  const [scenarioTab, setScenarioTab] = useState<ScenarioTab>('login');

  const tabDefs: {value: ScenarioTab; label: string; icon: JSX.Element; disabled?: boolean}[] = [
    {value: 'login', label: t('common:welcome.applicationTryout.scenarios.tabs.login'), icon: <LogIn size={15} />},
    {value: 'signup', label: t('common:welcome.applicationTryout.scenarios.tabs.signup'), icon: <UserPlus size={15} />},
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
    {
      value: 'mfa',
      label: t('common:welcome.applicationTryout.scenarios.tabs.mfa'),
      icon: <ShieldCheck size={15} />,
      disabled: true,
    },
    {
      value: 'social',
      label: t('common:welcome.applicationTryout.scenarios.tabs.social'),
      icon: <Share2 size={15} />,
      disabled: true,
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
                {
                  key: 'welcome',
                  label: t('common:welcome.header'),
                  onClick: () => void navigate(RouteConfig.welcome.root()),
                },
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
                  <Tab
                    key={tab.value}
                    value={tab.value}
                    icon={tab.icon}
                    iconPosition="start"
                    disabled={tab.disabled}
                    label={
                      tab.disabled ? (
                        <Box sx={{display: 'flex', alignItems: 'center', gap: 0.75}}>
                          {tab.label}
                          <Chip
                            label={t('common:welcome.getStarted.options.comingSoon')}
                            size="small"
                            sx={{height: 16, fontSize: '0.6rem', pointerEvents: 'none'}}
                          />
                        </Box>
                      ) : (
                        tab.label
                      )
                    }
                  />
                ))}
              </Tabs>

              <Box sx={{p: 3}}>
                {scenarioTab === 'login' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.login.description', {productName})}
                    </Typography>
                    <StepList
                      steps={[
                        <Trans
                          key="step1"
                          ns="common"
                          i18nKey="welcome.applicationTryout.scenarios.login.step1"
                          components={{
                            a: <ExternalLink href={WAYFINDER_APP_URL} />,
                            mail: <ExternalLink href={WAYFINDER_MAIL_URL} />,
                          }}
                        />,
                        t('common:welcome.applicationTryout.scenarios.login.step2'),
                      ]}
                    />
                    <CredentialsBlock username="john.doe" password="john.doe" />

                    <Divider />

                    <Typography variant="subtitle2" fontWeight={600}>
                      {t('common:welcome.applicationTryout.scenarios.tabs.profile')}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.profile.description')}
                    </Typography>
                    <StepList
                      startFrom={3}
                      steps={[
                        t('common:welcome.applicationTryout.scenarios.profile.step2'),
                        t('common:welcome.applicationTryout.scenarios.profile.step3', {productName}),
                      ]}
                    />
                  </Stack>
                )}

                {scenarioTab === 'signup' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.signup.description', {productName})}
                    </Typography>
                    <StepList
                      steps={[
                        <Trans
                          key="step1"
                          ns="common"
                          i18nKey="welcome.applicationTryout.scenarios.signup.step1"
                          components={{
                            a: <ExternalLink href={WAYFINDER_APP_URL} />,
                            mail: <ExternalLink href={WAYFINDER_MAIL_URL} />,
                          }}
                        />,
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
                          label: t('common:welcome.applicationTryout.scenarios.signup.sampleFields.password'),
                          value: 'emma.wilson',
                          isPassword: true,
                        },
                      ]}
                    />
                    <StepList startFrom={4} steps={[t('common:welcome.applicationTryout.scenarios.signup.step4')]} />
                    <FormFieldsBlock
                      fields={[
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
                      startFrom={5}
                      steps={[t('common:welcome.applicationTryout.scenarios.signup.step5', {productName})]}
                    />
                  </Stack>
                )}

                {scenarioTab === 'recovery' && (
                  <Stack spacing={2}>
                    <Typography variant="body2" color="text.secondary">
                      {t('common:welcome.applicationTryout.scenarios.recovery.description')}
                    </Typography>
                    <StepList
                      steps={[
                        <Trans
                          key="step1"
                          ns="common"
                          i18nKey="welcome.applicationTryout.scenarios.recovery.step1"
                          components={{
                            a: <ExternalLink href={WAYFINDER_APP_URL} />,
                            mail: <ExternalLink href={WAYFINDER_MAIL_URL} />,
                          }}
                        />,
                        t('common:welcome.applicationTryout.scenarios.recovery.step2', {productName}),
                        <Trans
                          key="step3"
                          ns="common"
                          i18nKey="welcome.applicationTryout.scenarios.recovery.step3"
                          components={{code: <CodeInline />}}
                        />,
                        <Trans
                          key="step4"
                          ns="common"
                          i18nKey="welcome.applicationTryout.scenarios.recovery.step4"
                          values={{productName}}
                          components={{
                            a: <ExternalLink href={WAYFINDER_APP_URL} />,
                            mail: <ExternalLink href={WAYFINDER_MAIL_URL} />,
                          }}
                        />,
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
                        <Trans
                          key="step5"
                          ns="common"
                          i18nKey="welcome.applicationTryout.scenarios.onboard.step5"
                          components={{
                            a: <ExternalLink href={WAYFINDER_APP_URL} />,
                            mail: <ExternalLink href={WAYFINDER_MAIL_URL} />,
                          }}
                        />,
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
