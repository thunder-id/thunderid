/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Box, Typography, Stack, TextField, IconButton, InputAdornment, Alert, Avatar, Paper} from '@wso2/oxygen-ui';
import {Copy, Eye, EyeOff, Check} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState, useEffect, useRef} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../../../../configs/RouteConfig';
import type {IntegrationGuides} from '../../models/application-templates';
import TechnologyGuide from '../edit-application/integration-guides/TechnologyGuide';

/**
 * Props for the {@link IntegrationGuide} component.
 *
 * @public
 */
export interface IntegrationGuideProps {
  /**
   * The name of the created application
   */
  appName: string;
  /**
   * URL of the application logo
   */
  appLogo: string | null;
  /**
   * The primary color used for branding
   */
  selectedColor: string;
  /**
   * The client ID (if OAuth was configured)
   */
  clientId?: string;
  /**
   * The client secret (if OAuth was configured)
   */
  clientSecret?: string;
  /**
   * Whether OAuth configs were selected in previous step
   */
  hasOAuthConfig: boolean;
  /**
   * The ID of the created application
   */
  applicationId?: string | null;
  /**
   * Integration guides configuration (optional - if not provided, won't show guides)
   */
  integrationGuides?: IntegrationGuides | null;
}

/**
 * React component that displays integration guides and setup instructions
 * for newly created applications.
 *
 * This component provides:
 * 1. Technology-specific integration guides with code snippets
 * 2. OAuth2 credentials (Client ID and Secret) when applicable
 * 3. Step-by-step instructions for integrating with various frameworks
 *
 * The component handles different scenarios:
 * - Applications with integration guides (shows TechnologyGuide)
 * - Applications with OAuth configuration (displays credentials)
 * - Public vs confidential client configurations
 *
 * @param props - The component props
 * @param props.appName - Name of the application
 * @param props.appLogo - URL of the application logo
 * @param props.selectedColor - Brand color for visual elements
 * @param props.clientId - OAuth2 client ID (if applicable)
 * @param props.clientSecret - OAuth2 client secret (if applicable)
 * @param props.hasOAuthConfig - Whether OAuth was configured
 * @param props.applicationId - ID of the application
 *
 * @returns JSX element displaying the integration guide
 *
 * @example
 * ```tsx
 * import IntegrationGuide from './IntegrationGuide';
 *
 * function ApplicationOverview() {
 *   return (
 *     <IntegrationGuide
 *       appName="My Application"
 *       appLogo="https://example.com/logo.png"
 *       selectedColor="#FF5733"
 *       clientId="abc123"
 *       clientSecret="secret456"
 *       hasOAuthConfig={true}
 *       applicationId="app-uuid"
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function IntegrationGuide({
  appName,
  appLogo,
  selectedColor,
  clientId = '',
  clientSecret = '',
  hasOAuthConfig,
  applicationId = null,
  integrationGuides = null,
}: IntegrationGuideProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();

  const [showSecret, setShowSecret] = useState(false);
  const [copied, setCopied] = useState<{clientId: boolean; clientSecret: boolean}>({
    clientId: false,
    clientSecret: false,
  });

  const copyTimeoutsRef = useRef<{
    clientId?: ReturnType<typeof setTimeout>;
    clientSecret?: ReturnType<typeof setTimeout>;
  }>({});

  /**
   * Clean up timeouts on unmount to prevent memory leaks
   */
  useEffect((): (() => void) => {
    const timeouts = copyTimeoutsRef.current;
    return (): void => {
      if (timeouts.clientId) {
        clearTimeout(timeouts.clientId);
      }
      if (timeouts.clientSecret) {
        clearTimeout(timeouts.clientSecret);
      }
    };
  }, []);

  const handleCopy = async (text: string, type: 'clientId' | 'clientSecret'): Promise<void> => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied((prev) => ({...prev, [type]: true}));

      // Clear existing timeout for this type if any
      if (copyTimeoutsRef.current[type]) {
        clearTimeout(copyTimeoutsRef.current[type]);
      }

      const timeoutId = setTimeout(() => {
        setCopied((prev) => ({...prev, [type]: false}));
        copyTimeoutsRef.current[type] = undefined;
      }, 2000);
      copyTimeoutsRef.current[type] = timeoutId;
    } catch {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.opacity = '0';
      document.body.appendChild(textArea);
      textArea.select();
      try {
        document.execCommand('copy');
        setCopied((prev) => ({...prev, [type]: true}));

        // Clear existing timeout for this type if any
        if (copyTimeoutsRef.current[type]) {
          clearTimeout(copyTimeoutsRef.current[type]);
        }

        const timeoutId = setTimeout(() => {
          setCopied((prev) => ({...prev, [type]: false}));
          copyTimeoutsRef.current[type] = undefined;
        }, 2000);
        copyTimeoutsRef.current[type] = timeoutId;
      } catch {
        // Ignore copy errors
      }
      document.body.removeChild(textArea);
    }
  };

  const handleToggleVisibility = (): void => {
    setShowSecret(!showSecret);
  };

  return (
    <Stack direction="column" spacing={4} sx={{maxWidth: 900, width: '100%', alignItems: 'center'}}>
      {/* Success Header */}
      <Stack direction="column" spacing={2} alignItems="center" sx={{width: '100%'}}>
        <Box
          role="img"
          aria-label="Success"
          sx={{
            width: 80,
            height: 80,
            borderRadius: '50%',
            bgcolor: 'success.main',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            mb: 2,
          }}
        >
          <Check size={48} color="white" aria-hidden="true" />
        </Box>
        <Typography variant="h3" component="h1" gutterBottom>
          {t('applications:onboarding.summary.title')}
        </Typography>
        {integrationGuides ? (
          <Typography variant="subtitle1">{t('applications:onboarding.summary.guides.subtitle')}</Typography>
        ) : (
          <Typography variant="subtitle1">{t('applications:onboarding.summary.subtitle')}</Typography>
        )}
      </Stack>

      {!integrationGuides && (
        <>
          <Paper
            sx={{
              p: 3,
              bgcolor: 'background.paper',
              width: '100%',
              cursor: applicationId ? 'pointer' : 'default',
              '&:hover': applicationId
                ? {
                    boxShadow: 2,
                    transition: 'box-shadow 0.2s ease-in-out',
                  }
                : {},
            }}
            role={applicationId ? 'button' : undefined}
            tabIndex={applicationId ? 0 : -1}
            aria-label={applicationId ? t('applications:onboarding.summary.viewAppAriaLabel') : undefined}
            onClick={(): void => {
              if (applicationId) {
                (async () => {
                  await navigate(RouteConfig.applications.detail(applicationId));
                })().catch(() => {
                  // Ignore navigation errors
                });
              }
            }}
            onKeyDown={
              applicationId
                ? (e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      (async () => {
                        await navigate(RouteConfig.applications.detail(applicationId));
                      })().catch(() => {
                        // Ignore navigation errors
                      });
                    }
                  }
                : undefined
            }
          >
            <Stack direction="row" spacing={3} alignItems="center">
              {appLogo ? (
                <Avatar src={appLogo} alt={`${appName} logo`} sx={{width: 64, height: 64, bgcolor: selectedColor}} />
              ) : (
                <Avatar sx={{width: 64, height: 64, bgcolor: selectedColor, fontSize: '1.5rem'}}>
                  {appName.charAt(0).toUpperCase()}
                </Avatar>
              )}
              <Box sx={{flex: 1}}>
                <Typography variant="h5" component="h2" gutterBottom>
                  {appName}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {t('applications:onboarding.summary.appDetails')}
                </Typography>
              </Box>
            </Stack>
          </Paper>

          {/* OAuth Credentials Section */}
          {hasOAuthConfig && clientId && (
            <Box sx={{width: '100%', textAlign: 'left'}}>
              {/* Only show warning if client secret exists (confidential clients) */}
              {clientSecret && (
                <Alert severity="warning" sx={{mb: 3}}>
                  {t('applications:clientSecret.warning')}
                </Alert>
              )}

              <Stack direction="column" spacing={2}>
                <Box>
                  <Typography variant="body2" color="text.secondary" sx={{mb: 1, textAlign: 'left'}}>
                    {t('applications:clientSecret.clientIdLabel')}
                  </Typography>
                  <TextField
                    fullWidth
                    value={clientId}
                    InputProps={{
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          <IconButton
                            onClick={() => {
                              handleCopy(clientId, 'clientId').catch(() => {
                                // Error already handled in handleCopy
                              });
                            }}
                            edge="end"
                            size="small"
                          >
                            <Copy size={16} />
                          </IconButton>
                        </InputAdornment>
                      ),
                    }}
                  />
                  {copied.clientId && (
                    <Typography variant="caption" color="success.main" sx={{mt: 0.5, display: 'block'}}>
                      {t('applications:clientSecret.copied')}
                    </Typography>
                  )}
                </Box>

                {/* Only show client secret for confidential clients (public clients don't have secrets) */}
                {clientSecret && (
                  <Box>
                    <Typography variant="body2" color="text.secondary" sx={{mb: 1, textAlign: 'left'}}>
                      {t('applications:clientSecret.clientSecretLabel')}
                    </Typography>
                    <TextField
                      fullWidth
                      type={showSecret ? 'text' : 'password'}
                      value={clientSecret}
                      InputProps={{
                        readOnly: true,
                        endAdornment: (
                          <InputAdornment position="end">
                            <IconButton onClick={handleToggleVisibility} edge="end" size="small">
                              {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                            </IconButton>
                            <IconButton
                              onClick={() => {
                                handleCopy(clientSecret, 'clientSecret').catch(() => {
                                  // Error already handled in handleCopy
                                });
                              }}
                              edge="end"
                              size="small"
                              sx={{ml: 0.5}}
                            >
                              <Copy size={16} />
                            </IconButton>
                          </InputAdornment>
                        ),
                      }}
                    />
                    {copied.clientSecret && (
                      <Typography variant="caption" color="success.main" sx={{mt: 0.5, display: 'block'}}>
                        {t('applications:clientSecret.copied')}
                      </Typography>
                    )}
                  </Box>
                )}
              </Stack>
            </Box>
          )}
        </>
      )}

      {/* Technology Integration Guides */}
      {integrationGuides && (
        <TechnologyGuide guides={integrationGuides} clientId={clientId} applicationId={applicationId ?? undefined} />
      )}
    </Stack>
  );
}
