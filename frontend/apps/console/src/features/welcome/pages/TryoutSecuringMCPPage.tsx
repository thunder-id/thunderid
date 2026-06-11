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
import {Box, Button, Stack, Tab, Tabs, Typography, IconButton, LinearProgress} from '@wso2/oxygen-ui';
import {
  BookOpen,
  Check,
  Copy,
  ExternalLink,
  Link2,
  Play,
  ShieldCheck,
  Terminal,
  MCP,
  X,
} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX, ReactNode} from 'react';
import {useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import TerminalBlock from '../components/TerminalBlock';
import WayfinderSampleSetup from '../components/WayfinderSampleSetup';
import useWelcomeClose from '../hooks/useWelcomeClose';
import AppBreadcrumbs from '@/components/AppBreadcrumbs';

const MotionBox = motion.create(Box);

type ScenarioTab = 'connect' | 'permissions';

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
          <Typography variant="caption" color="text.secondary" sx={{minWidth: 100, flexShrink: 0}}>
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

function ExternalAppLink({href, children = null}: {href: string; children?: ReactNode}): JSX.Element {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      style={{color: 'inherit', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: 2}}
    >
      {children}
      <ExternalLink size={12} style={{flexShrink: 0, opacity: 0.7}} />
    </a>
  );
}

function tLink(i18nKey: string, href: string): JSX.Element {
  return <Trans ns="common" i18nKey={i18nKey} components={{a: <ExternalAppLink href={href} />}} />;
}

function CodeBlock({code}: {code: string}): JSX.Element {
  const [copied, setCopied] = useState(false);
  const handleCopy = (): void => {
    void navigator.clipboard.writeText(code).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };
  return (
    <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden'}}>
      <Box
        sx={{
          bgcolor: 'action.selected',
          px: 1.5,
          py: 0.5,
          display: 'flex',
          justifyContent: 'flex-end',
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      >
        <IconButton
          size="small"
          aria-label="Copy snippet"
          onClick={handleCopy}
          sx={{color: copied ? 'success.main' : 'text.secondary'}}
        >
          {copied ? <Check size={13} /> : <Copy size={13} />}
        </IconButton>
      </Box>
      <Box sx={{bgcolor: 'grey.900', px: 2, py: 1.5}}>
        {code.split('\n').map((line) => (
          <Typography
            key={line.toLowerCase().split(' ').join('-')}
            variant="body2"
            fontFamily="monospace"
            sx={{color: 'grey.300', whiteSpace: 'pre'}}
          >
            {line}
          </Typography>
        ))}
      </Box>
    </Box>
  );
}

export default function TryoutSecuringMCPPage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const handleClose = useWelcomeClose();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.documentation?.baseUrl ?? '').replace(/\/$/, '');

  const [scenarioTab, setScenarioTab] = useState<ScenarioTab>('connect');

  const corsSnippet = `cors:\n  allowed_origins:\n    # ...existing entries...\n    - "http://localhost:6274"`;

  const tabDefs: {value: ScenarioTab; label: string; icon: JSX.Element}[] = [
    {value: 'connect', label: t('common:welcome.mcpTryout.scenarios.tabs.connect'), icon: <Link2 size={15} />},
    {
      value: 'permissions',
      label: t('common:welcome.mcpTryout.scenarios.tabs.permissions'),
      icon: <ShieldCheck size={15} />,
    },
  ];

  const setupSteps = [
    {
      number: 4,
      icon: <Terminal size={28} />,
      title: t('common:welcome.mcpTryout.steps.installInspector.title'),
      description: t('common:welcome.mcpTryout.steps.installInspector.description'),
    },
    {
      number: 5,
      icon: <Play size={28} />,
      title: t('common:welcome.mcpTryout.steps.allowCors.title'),
      description: t('common:welcome.mcpTryout.steps.allowCors.description', {productName}),
    },
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
                {key: 'tryout', label: t('common:welcome.mcpTryout.breadcrumb')},
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
                <MCP size={13} />
                {t('common:welcome.mcpTryout.overline')}
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
                {t('common:welcome.mcpTryout.subtitle', {productName})}
              </Typography>
            </Box>

            <WayfinderSampleSetup />

            {/* MCP-specific setup steps */}
            <Stack spacing={3} sx={{mt: 3}}>
              {setupSteps.map((step, index) => (
                <MotionBox
                  key={step.number}
                  initial={{opacity: 0, x: -10}}
                  animate={{opacity: 1, x: 0}}
                  transition={{duration: 0.3, delay: 0.2 + index * 0.1}}
                >
                  <Box
                    sx={{
                      display: 'flex',
                      gap: 3,
                      p: 3,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 2,
                      alignItems: 'flex-start',
                    }}
                  >
                    <Box
                      sx={{
                        width: 36,
                        height: 36,
                        borderRadius: '50%',
                        bgcolor: 'action.selected',
                        color: 'text.primary',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontWeight: 700,
                        fontSize: '0.875rem',
                        flexShrink: 0,
                        mt: 0.25,
                      }}
                    >
                      {step.number}
                    </Box>
                    <Box sx={{flex: 1}}>
                      <Box
                        sx={{
                          width: 48,
                          height: 48,
                          borderRadius: 2,
                          bgcolor: 'action.selected',
                          color: 'text.secondary',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          mb: 1.5,
                        }}
                      >
                        {step.icon}
                      </Box>
                      <Typography variant="subtitle1" fontWeight={600} sx={{mb: 0.5}}>
                        {step.title}
                      </Typography>
                      <Typography variant="body2" color="text.secondary" sx={{mb: step.number === 1 ? 1.5 : 0}}>
                        {step.description}
                      </Typography>

                      {step.number === 4 && <TerminalBlock command="npx @modelcontextprotocol/inspector" />}

                      {step.number === 5 && <CodeBlock code={corsSnippet} />}
                    </Box>
                  </Box>
                </MotionBox>
              ))}
            </Stack>

            {/* Try use cases */}
            <MotionBox
              initial={{opacity: 0, y: 10}}
              animate={{opacity: 1, y: 0}}
              transition={{duration: 0.4, delay: 0.5}}
              sx={{mt: 4}}
            >
              <Typography variant="h3" sx={{fontSize: '1.25rem', fontWeight: 600, mb: 2}}>
                {t('common:welcome.mcpTryout.scenarios.title')}
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
                  {scenarioTab === 'connect' && (
                    <Stack spacing={2}>
                      <Typography variant="body2" color="text.secondary">
                        {t('common:welcome.mcpTryout.scenarios.connect.description', {productName})}
                      </Typography>
                      <StepList
                        steps={[
                          tLink('welcome.mcpTryout.scenarios.connect.step1', 'http://localhost:6274'),
                          t('common:welcome.mcpTryout.scenarios.connect.step2'),
                          t('common:welcome.mcpTryout.scenarios.connect.step3'),
                          t('common:welcome.mcpTryout.scenarios.connect.step4', {productName}),
                          t('common:welcome.mcpTryout.scenarios.connect.step5'),
                        ]}
                      />
                      <Typography variant="caption" color="text.secondary">
                        {t('common:welcome.mcpTryout.scenarios.connect.connectionLabel')}
                      </Typography>
                      <FormFieldsBlock
                        fields={[
                          {
                            label: t('common:welcome.mcpTryout.scenarios.connect.fields.transport'),
                            value: 'Streamable HTTP',
                          },
                          {
                            label: t('common:welcome.mcpTryout.scenarios.connect.fields.serverUrl'),
                            value: 'http://localhost:8787/mcp',
                          },
                          {
                            label: t('common:welcome.mcpTryout.scenarios.connect.fields.clientId'),
                            value: 'EXTERNAL-MCP-CLIENT',
                          },
                          {
                            label: t('common:welcome.mcpTryout.scenarios.connect.fields.clientSecret'),
                            value: '(leave blank)',
                          },
                        ]}
                      />
                    </Stack>
                  )}

                  {scenarioTab === 'permissions' && (
                    <Stack spacing={2}>
                      <Typography variant="body2" color="text.secondary">
                        {t('common:welcome.mcpTryout.scenarios.permissions.description', {productName})}
                      </Typography>
                      <StepList
                        steps={[
                          t('common:welcome.mcpTryout.scenarios.permissions.step1'),
                          t('common:welcome.mcpTryout.scenarios.permissions.step2'),
                          t('common:welcome.mcpTryout.scenarios.permissions.step3'),
                          t('common:welcome.mcpTryout.scenarios.permissions.step4'),
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
              transition={{duration: 0.4, delay: 0.7}}
              sx={{mt: 4, display: 'flex', justifyContent: 'center'}}
            >
              <Button
                variant="text"
                size="small"
                startIcon={<BookOpen size={16} />}
                onClick={() =>
                  window.open(
                    `${docsBaseUrl}/use-cases/ai-agents/mcp-authorization/try-it-out`,
                    '_blank',
                    'noopener,noreferrer',
                  )
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
