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

import {
  Dialog,
  DialogContent,
  DialogActions,
  Button,
  Alert,
  Box,
  Typography,
  TextField,
  IconButton,
  InputAdornment,
  Stack,
} from '@wso2/oxygen-ui';
import {Copy, Eye, EyeOff, AlertTriangle} from '@wso2/oxygen-ui-icons-react';
import {useState, useRef, useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link ClientSecretSuccessDialog} component.
 */
export interface ClientSecretSuccessDialogProps {
  /**
   * Whether the dialog is open
   */
  open: boolean;
  /**
   * The new secret to display
   */
  clientSecret: string;
  /**
   * Optional override for the dialog title. Defaults to the client secret title.
   */
  title?: string;
  /**
   * Optional override for the dialog subtitle. Defaults to the client secret subtitle.
   */
  subtitle?: string;
  /**
   * Optional override for the secret field label. Defaults to the client secret label.
   */
  secretLabel?: string;
  /**
   * Optional override for the primary copy button label. Defaults to the client secret label.
   */
  copySecretLabel?: string;
  /**
   * Optional override for the security reminder title. Defaults to the client secret reminder.
   */
  securityReminderTitle?: string;
  /**
   * Optional override for the security reminder description. Defaults to the client secret reminder.
   */
  securityReminderDescription?: string;
  /**
   * Callback when the dialog should be closed
   */
  onClose: () => void;
}

/**
 * Dialog component for displaying the new client secret after successful regeneration.
 *
 * This dialog shows the new client secret with a copy button and warns users
 * that the secret will not be shown again after closing the dialog.
 *
 * @param props - Component props
 * @returns The client secret success dialog
 */
export default function ClientSecretSuccessDialog({
  open,
  clientSecret,
  title = undefined,
  subtitle = undefined,
  secretLabel = undefined,
  copySecretLabel = undefined,
  securityReminderTitle = undefined,
  securityReminderDescription = undefined,
  onClose,
}: ClientSecretSuccessDialogProps): JSX.Element {
  const {t} = useTranslation();
  const [copied, setCopied] = useState(false);
  const [showSecret, setShowSecret] = useState(false);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  // Clear the copy timeout on unmount to prevent state updates after unmount
  useEffect(() => () => clearTimeout(copyTimeoutRef.current), []);

  const handleCopy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(clientSecret);
      setCopied(true);
      clearTimeout(copyTimeoutRef.current);
      copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      // Failed to copy - user can manually select and copy
    }
  };

  const handleToggleVisibility = (): void => {
    setShowSecret(!showSecret);
  };

  const handleClose = (): void => {
    clearTimeout(copyTimeoutRef.current);
    setCopied(false);
    setShowSecret(false);
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogContent>
        <Stack direction="column" spacing={3} sx={{width: '100%', pt: 2}}>
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
            <Typography variant="h5" component="h2">
              {title ?? t('applications:regenerateSecret.success.title')}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {subtitle ?? t('applications:regenerateSecret.success.subtitle')}
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
            <Box>
              <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
                {secretLabel ?? t('applications:regenerateSecret.success.secretLabel')}
              </Typography>
              <TextField
                fullWidth
                type={showSecret ? 'text' : 'password'}
                value={clientSecret}
                slotProps={{
                  input: {
                    readOnly: true,
                    endAdornment: (
                      <InputAdornment position="end">
                        <IconButton
                          onClick={handleToggleVisibility}
                          edge="end"
                          size="small"
                          aria-label={t('applications:regenerateSecret.success.toggleVisibility')}
                        >
                          {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                        </IconButton>
                        <IconButton
                          onClick={() => {
                            handleCopy().catch(() => {
                              // Error is handled silently
                            });
                          }}
                          edge="end"
                          size="small"
                          sx={{ml: 0.5}}
                          aria-label={t('applications:regenerateSecret.success.copyButton')}
                        >
                          <Copy size={16} />
                        </IconButton>
                      </InputAdornment>
                    ),
                    sx: {
                      fontFamily: 'monospace',
                      fontSize: '0.875rem',
                    },
                  },
                }}
              />
            </Box>
          </Box>

          {/* Security Reminder Alert */}
          <Alert severity="warning" icon={<AlertTriangle size={20} />}>
            <Typography variant="body2" sx={{fontWeight: 'medium', mb: 1}}>
              {securityReminderTitle ?? t('applications:regenerateSecret.success.securityReminder.title')}
            </Typography>
            <Typography variant="body2">
              {securityReminderDescription ?? t('applications:regenerateSecret.success.securityReminder.description')}
            </Typography>
          </Alert>
        </Stack>
      </DialogContent>
      <DialogActions sx={{px: 3, pb: 3, pt: 1}}>
        <Stack direction="row" spacing={2} sx={{width: '100%'}}>
          <Button
            variant="contained"
            fullWidth
            startIcon={<Copy size={16} />}
            onClick={() => {
              handleCopy().catch(() => {
                // Error is handled silently
              });
            }}
            disabled={copied}
          >
            {copied
              ? t('applications:regenerateSecret.success.copied')
              : (copySecretLabel ?? t('applications:regenerateSecret.success.copySecret'))}
          </Button>
          <Button variant="outlined" fullWidth onClick={handleClose}>
            {t('common:actions.done')}
          </Button>
        </Stack>
      </DialogActions>
    </Dialog>
  );
}
