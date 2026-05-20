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

import {useCopyToClipboard} from '@thunderid/hooks';
import {Box, Typography, Stack, TextField, IconButton, InputAdornment, Alert, Button, Divider} from '@wso2/oxygen-ui';
import {Copy, Eye, EyeOff, AlertTriangle} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';

export interface ShowClientSecretProps {
  /**
   * The name of the created application
   */
  appName: string;
  /**
   * The client ID of the created application
   */
  clientId?: string;
  /**
   * The client secret that needs to be saved
   */
  clientSecret: string;
  /**
   * Callback when user clicks copy secret button
   */
  onCopySecret?: () => void;
  /**
   * Callback when user clicks continue button
   */
  onContinue: () => void;
}

/**
 * Component that displays the client secret that needs to be saved
 * with security reminders and educational content
 */
export default function ShowClientSecret({
  appName,
  clientId = '',
  clientSecret,
  onCopySecret = () => null,
  onContinue,
}: ShowClientSecretProps): JSX.Element {
  const {t} = useTranslation();
  const [showSecret, setShowSecret] = useState(false);
  const {copy: copyClientId} = useCopyToClipboard({
    resetDelay: 2000,
  }) as {copied: boolean; copy: (text: string) => Promise<void>};
  const {copied, copy} = useCopyToClipboard({
    resetDelay: 2000,
    onCopy: onCopySecret,
  }) as {copied: boolean; copy: (text: string) => Promise<void>};

  const handleClientIdCopy = async (): Promise<void> => {
    await copyClientId(clientId);
  };

  const handleCopy = async (): Promise<void> => {
    await copy(clientSecret);
  };

  const handleToggleVisibility = (): void => {
    setShowSecret(!showSecret);
  };

  return (
    <Stack direction="column" spacing={4} sx={{width: '100%'}} data-testid="application-show-client-secret">
      {/* Warning Icon */}
      <Box
        sx={{
          width: 64,
          height: 64,
          borderRadius: 2,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          alignSelf: 'center',
        }}
      >
        <AlertTriangle size={64} color="var(--mui-palette-warning-main)" />
      </Box>

      {/* Header */}
      <Stack direction="column" spacing={1} sx={{textAlign: 'center'}}>
        <Typography variant="h3" component="h1">
          {t('applications:clientSecret.saveTitle')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t('applications:clientSecret.saveSubtitle')}
        </Typography>
      </Stack>

      {/* Application Name & Client Secret Card */}
      <Box
        sx={{
          p: 3,
          bgcolor: 'background.paper',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
        }}
      >
        <Stack direction="column" spacing={2}>
          <Box>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
              {t('applications:clientSecret.appNameLabel')}
            </Typography>
            <Typography variant="body1">{appName}</Typography>
          </Box>

          {clientId && (
            <>
              <Divider />

              <Box>
                <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
                  {t('applications:clientSecret.clientIdLabel')}
                </Typography>
                <TextField
                  fullWidth
                  data-testid="application-client-id-value"
                  value={clientId}
                  InputProps={{
                    readOnly: true,
                    endAdornment: (
                      <InputAdornment position="end">
                        <IconButton
                          aria-label={`${t('common:actions.copy')} ${t('applications:clientSecret.clientIdLabel')}`}
                          onClick={() => {
                            handleClientIdCopy().catch(() => {
                              // Error already handled in handleClientIdCopy
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
              </Box>
            </>
          )}

          <Divider />

          <Box>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
              {t('applications:clientSecret.clientSecretLabel')}
            </Typography>
            <TextField
              fullWidth
              data-testid="application-client-secret-value"
              type={showSecret ? 'text' : 'password'}
              value={clientSecret}
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      aria-label={t('applications:regenerateSecret.success.toggleVisibility')}
                      onClick={handleToggleVisibility}
                      edge="end"
                      size="small"
                    >
                      {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                    </IconButton>
                    <IconButton
                      aria-label={`${t('common:actions.copy')} ${t('applications:clientSecret.clientSecretLabel')}`}
                      onClick={() => {
                        handleCopy().catch(() => {
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
          </Box>
        </Stack>
      </Box>

      {/* Security Reminder Alert */}
      <Alert severity="warning" icon={<AlertTriangle size={20} />}>
        <Typography variant="body2" sx={{fontWeight: 'medium', mb: 1}}>
          {t('applications:clientSecret.securityReminder.title')}
        </Typography>
        <Typography variant="body2">{t('applications:clientSecret.securityReminder.description')}</Typography>
      </Alert>

      {/* Action Buttons */}
      <Stack direction="row" spacing={2} sx={{width: '100%'}}>
        <Button
          variant="contained"
          fullWidth
          startIcon={<Copy size={16} />}
          onClick={() => {
            handleCopy().catch(() => {
              // Error already handled in handleCopy
            });
          }}
          disabled={copied}
        >
          {copied ? t('applications:clientSecret.copied') : t('applications:clientSecret.copySecret')}
        </Button>
        <Button data-testid="application-client-secret-continue" variant="outlined" fullWidth onClick={onContinue}>
          {t('common:actions.continue')}
        </Button>
      </Stack>
    </Stack>
  );
}
