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
import {Box, Button, Divider, Stack, Typography, IconButton, LinearProgress, AppBreadcrumbs} from '@wso2/oxygen-ui';
import {BookOpen, Terminal, MCP, X} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import CodeInline from '../components/CodeInline';
import CredentialsBlock from '../components/CredentialsBlock';
import ExternalLink from '../components/ExternalLink';
import FormFieldsBlock from '../components/FormFieldsBlock';
import StepList from '../components/StepList';
import TerminalBlock from '../components/TerminalBlock';
import WayfinderSampleSetup from '../components/WayfinderSampleSetup';
import {MCP_INSPECTOR_CALLBACK_URL, MCP_INSPECTOR_URL, WAYFINDER_MCP_URL} from '../constants/sample-urls';
import useWelcomeClose from '../hooks/useWelcomeClose';

const MotionBox = motion.create(Box);

export default function TryoutSecuringMCPPage(): JSX.Element {
  const {t} = useTranslation(['common']);
  const navigate = useNavigate();
  const {config} = useConfig();
  const handleClose = useWelcomeClose();
  const productName = config.brand.product_name;
  const docsBaseUrl = (config.brand.documentation?.baseUrl ?? '').replace(/\/$/, '');

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

            {/* Step 4 — Launch MCP Inspector */}
            <MotionBox
              initial={{opacity: 0, x: -10}}
              animate={{opacity: 1, x: 0}}
              transition={{duration: 0.3, delay: 0.2}}
              sx={{mt: 3}}
            >
              <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden'}}>
                <Box
                  sx={{
                    p: 2.5,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 2,
                    bgcolor: 'action.selected',
                    borderBottom: '1px solid',
                    borderColor: 'divider',
                  }}
                >
                  <Box
                    sx={{
                      width: 40,
                      height: 40,
                      borderRadius: 2,
                      bgcolor: 'background.paper',
                      color: 'text.secondary',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0,
                    }}
                  >
                    <Terminal size={22} />
                  </Box>
                  <Typography variant="subtitle2" fontWeight={600}>
                    {t('common:welcome.mcpTryout.steps.installInspector.title')}
                  </Typography>
                </Box>
                <Divider />
                <Box sx={{p: 2.5}}>
                  <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
                    {t('common:welcome.mcpTryout.steps.installInspector.description')}
                  </Typography>
                  <TerminalBlock command="npx @modelcontextprotocol/inspector" />
                </Box>
              </Box>
            </MotionBox>

            {/* Try use cases */}
            <MotionBox
              initial={{opacity: 0, y: 10}}
              animate={{opacity: 1, y: 0}}
              transition={{duration: 0.4, delay: 0.5}}
              sx={{mt: 5}}
            >
              <Typography variant="h3" sx={{fontSize: '1.25rem', fontWeight: 600, mb: 2}}>
                {t('common:welcome.mcpTryout.scenarios.title')}
              </Typography>

              <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, p: 3}}>
                <Stack spacing={2}>
                  <Typography variant="body2" color="text.secondary">
                    {t('common:welcome.mcpTryout.scenarios.connect.description', {productName})}
                  </Typography>
                  <StepList
                    steps={[
                      <Trans
                        key="step1"
                        ns="common"
                        i18nKey="welcome.mcpTryout.scenarios.connect.step1"
                        components={{a: <ExternalLink href={MCP_INSPECTOR_URL} />}}
                      />,
                      t('common:welcome.mcpTryout.scenarios.connect.step2'),
                    ]}
                  />
                  <FormFieldsBlock
                    fields={[
                      {
                        label: t('common:welcome.mcpTryout.scenarios.connect.fields.transport'),
                        value: 'Streamable HTTP',
                        readOnly: true,
                      },
                      {
                        label: t('common:welcome.mcpTryout.scenarios.connect.fields.serverUrl'),
                        value: WAYFINDER_MCP_URL,
                      },
                      {
                        label: t('common:welcome.mcpTryout.scenarios.connect.fields.connectionType'),
                        value: 'Direct',
                        readOnly: true,
                      },
                    ]}
                  />
                  <StepList startFrom={3} steps={[t('common:welcome.mcpTryout.scenarios.connect.step3')]} />
                  <FormFieldsBlock
                    fields={[
                      {
                        label: t('common:welcome.mcpTryout.scenarios.connect.fields.clientId'),
                        value: 'EXTERNAL-MCP-CLIENT',
                      },
                      {
                        label: t('common:welcome.mcpTryout.scenarios.connect.fields.clientSecret'),
                        value: '(leave blank)',
                        readOnly: true,
                      },
                      {
                        label: t('common:welcome.mcpTryout.scenarios.connect.fields.redirectUrl'),
                        value: MCP_INSPECTOR_CALLBACK_URL,
                      },
                    ]}
                  />
                  <StepList
                    startFrom={4}
                    steps={[t('common:welcome.mcpTryout.scenarios.connect.step4', {productName})]}
                  />
                  <CredentialsBlock username="john.doe" password="john.doe" />
                  <StepList
                    startFrom={5}
                    steps={[
                      <Trans
                        key="step5"
                        ns="common"
                        i18nKey="welcome.mcpTryout.scenarios.connect.step5"
                        components={{code: <CodeInline />}}
                      />,
                    ]}
                  />
                </Stack>
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
