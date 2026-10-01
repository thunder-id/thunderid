// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCopyToClipboard} from '@thunderid/hooks';
import {
  Box,
  Typography,
  Stack,
  TextField,
  IconButton,
  InputAdornment,
  Alert,
  Button,
  Divider,
  useTheme,
} from '@wso2/oxygen-ui';
import {Copy, Eye, EyeOff, AlertTriangle} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * A translation key paired with its English fallback.
 */
type SecretCopy = [key: string, fallback: string];

export interface ShowClientSecretProps {
  /**
   * The OAuth client secret that needs to be saved. Used to authenticate at the token endpoint.
   */
  clientSecret?: string;
  /**
   * The Flow Secret that needs to be saved. Used to authenticate when initiating a flow directly
   * via the Flow Execution API. Issued to backend / server-side applications.
   */
  flowSecret?: string;
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
 * Component that displays the credentials (client secret and/or Flow Secret) that need to be saved
 * with security reminders and educational content. An application may have both: the client secret
 * authenticates at the OAuth token endpoint, while the Flow Secret authenticates direct flow
 * initiation via the Flow Execution API.
 */
export default function ShowClientSecret({
  clientSecret = '',
  flowSecret = '',
  onCopySecret = () => null,
  onContinue,
}: ShowClientSecretProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const [showSecret, setShowSecret] = useState(false);
  const [showFlowSecret, setShowFlowSecret] = useState(false);
  const {copied, copy} = useCopyToClipboard({
    resetDelay: 2000,
    onCopy: onCopySecret,
  }) as {copied: boolean; copy: (text: string) => Promise<void>};
  const {copy: copyFlowSecret} = useCopyToClipboard({
    resetDelay: 2000,
  }) as {copied: boolean; copy: (text: string) => Promise<void>};

  // The primary secret backs the footer copy button: prefer the client secret, fall back to the
  // Flow Secret for embedded apps that only have the latter.
  const primarySecret = clientSecret || flowSecret;
  const flowSecretOnly = Boolean(flowSecret) && !clientSecret;
  const bothSecrets = Boolean(clientSecret) && Boolean(flowSecret);

  const resolveCopy = (clientCopy: SecretCopy, flowCopy: SecretCopy, bothCopy: SecretCopy): string => {
    if (bothSecrets) {
      return t(bothCopy[0], bothCopy[1]);
    }
    const [key, fallback] = flowSecretOnly ? flowCopy : clientCopy;
    return t(key, fallback);
  };

  const saveTitle = resolveCopy(
    ['applications:clientSecret.saveTitle', 'Save Your Client Secret'],
    ['applications:flowSecret.saveTitle', 'Save Your Flow Secret'],
    ['applications:secrets.saveTitle', 'Save Your Secrets'],
  );
  const saveSubtitle = resolveCopy(
    [
      'applications:clientSecret.saveSubtitle',
      "This is the only time you'll see this secret. Store it somewhere safe.",
    ],
    ['applications:flowSecret.saveSubtitle', "This is the only time you'll see this secret. Store it somewhere safe."],
    ['applications:secrets.saveSubtitle', "This is the only time you'll see these secrets. Store them somewhere safe."],
  );
  const securityReminderDescription = resolveCopy(
    [
      'applications:clientSecret.securityReminder.description',
      'Your client secret is a confidential key used to authenticate your application. It should be treated with the same level of security as a password. Never expose it in browser console, version control, or logs.',
    ],
    [
      'applications:flowSecret.securityReminder.description',
      'Your Flow Secret is a confidential key used to authenticate your application when it starts a sign-in flow. It should be treated with the same level of security as a password. Never expose it in browser console, version control, or logs.',
    ],
    [
      'applications:secrets.securityReminder.description',
      'These secrets are confidential keys used to authenticate your application. They should be treated with the same level of security as passwords. Never expose them in browser console, version control, or logs.',
    ],
  );
  // When both secrets are shown the footer button copies the client secret, so it is labelled
  // explicitly; the Flow Secret has its own copy button on its field.
  const copySecretLabel = flowSecretOnly
    ? t('applications:flowSecret.copySecret', 'Copy Flow Secret')
    : t('applications:clientSecret.copySecret', 'Copy Client Secret');

  const handleCopy = async (): Promise<void> => {
    await copy(primarySecret);
  };

  const handleFlowSecretCopy = async (): Promise<void> => {
    await copyFlowSecret(flowSecret);
  };

  const handleToggleVisibility = (): void => {
    setShowSecret(!showSecret);
  };

  const handleToggleFlowSecretVisibility = (): void => {
    setShowFlowSecret(!showFlowSecret);
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
        <AlertTriangle size={64} color={theme.vars?.palette.warning.main} />
      </Box>

      {/* Header */}
      <Stack direction="column" spacing={1} sx={{textAlign: 'center'}}>
        <Typography variant="h3" component="h1">
          {saveTitle}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {saveSubtitle}
        </Typography>
      </Stack>

      {/* Client Secret Card */}
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
          {clientSecret && (
            <Box>
              <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
                {t('applications:clientSecret.clientSecretLabel', 'Client Secret')}
              </Typography>
              <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
                {t(
                  'applications:clientSecret.purpose',
                  'Used to authenticate your application at the OAuth 2 token endpoint.',
                )}
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
                        aria-label={t(
                          'applications:regenerateSecret.success.toggleVisibility',
                          'Toggle secret visibility',
                        )}
                        onClick={handleToggleVisibility}
                        edge="end"
                        size="small"
                      >
                        {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                      </IconButton>
                      <IconButton
                        aria-label={`${t('common:actions.copy', 'Copy')} ${t('applications:clientSecret.clientSecretLabel', 'Client Secret')}`}
                        onClick={() => {
                          copy(clientSecret).catch(() => {
                            // Error already handled in copy
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
          )}

          {flowSecret && (
            <>
              {clientSecret && <Divider />}

              <Box>
                <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
                  {t('applications:flowSecret.label', 'Flow Secret')}
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
                  {t(
                    'applications:flowSecret.purpose',
                    'Used to authenticate your server when it starts a sign-in flow directly via the Flow Execution API.',
                  )}
                </Typography>
                <TextField
                  fullWidth
                  data-testid="application-app-secret-value"
                  type={showFlowSecret ? 'text' : 'password'}
                  value={flowSecret}
                  InputProps={{
                    readOnly: true,
                    endAdornment: (
                      <InputAdornment position="end">
                        <IconButton
                          aria-label={t(
                            'applications:regenerateSecret.success.toggleVisibility',
                            'Toggle secret visibility',
                          )}
                          onClick={handleToggleFlowSecretVisibility}
                          edge="end"
                          size="small"
                        >
                          {showFlowSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                        </IconButton>
                        <IconButton
                          aria-label={`${t('common:actions.copy', 'Copy')} ${t('applications:flowSecret.label', 'Flow Secret')}`}
                          onClick={() => {
                            handleFlowSecretCopy().catch(() => {
                              // Error already handled in handleFlowSecretCopy
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
            </>
          )}
        </Stack>
      </Box>

      {/* Security Reminder Alert */}
      <Alert severity="warning" icon={<AlertTriangle size={20} />}>
        <Typography variant="body2" sx={{fontWeight: 'medium', mb: 1}}>
          {t('applications:clientSecret.securityReminder.title', 'Security Reminder')}
        </Typography>
        <Typography variant="body2">{securityReminderDescription}</Typography>
      </Alert>

      {/* Action Buttons */}
      <Stack direction="row" spacing={2} sx={{width: '100%'}}>
        <Button
          data-testid="application-copy-secret-button"
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
          {copied ? t('applications:clientSecret.copied', 'Copied to clipboard') : copySecretLabel}
        </Button>
        <Button data-testid="application-client-secret-continue" variant="outlined" fullWidth onClick={onContinue}>
          {t('common:actions.continue', 'Continue')}
        </Button>
      </Stack>
    </Stack>
  );
}
