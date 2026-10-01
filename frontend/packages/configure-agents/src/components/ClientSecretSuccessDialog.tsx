// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import {AlertTriangle, Copy, Eye, EyeOff} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface ClientSecretSuccessDialogProps {
  open: boolean;
  clientSecret: string;
  onClose: () => void;
}

export default function ClientSecretSuccessDialog({
  open,
  clientSecret,
  onClose,
}: ClientSecretSuccessDialogProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const [copied, setCopied] = useState(false);
  const [showSecret, setShowSecret] = useState(false);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => () => clearTimeout(copyTimeoutRef.current), []);

  const handleCopy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(clientSecret);
      setCopied(true);
      clearTimeout(copyTimeoutRef.current);
      copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore
    }
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

          <Stack direction="column" spacing={1} sx={{textAlign: 'center'}}>
            <Typography variant="h5" component="h2">
              {t('agents:regenerateSecret.success.title', 'Client secret regenerated')}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t(
                'agents:regenerateSecret.success.subtitle',
                "Copy your new client secret and store it somewhere safe. It won't be shown again.",
              )}
            </Typography>
          </Stack>

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
                {t('agents:regenerateSecret.success.secretLabel', 'New Client Secret')}
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
                        <IconButton onClick={() => setShowSecret(!showSecret)} edge="end" size="small">
                          {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                        </IconButton>
                        <IconButton
                          onClick={() => {
                            handleCopy().catch(() => null);
                          }}
                          edge="end"
                          size="small"
                          sx={{ml: 0.5}}
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

          <Alert severity="warning" icon={<AlertTriangle size={20} />}>
            <Typography variant="body2" sx={{fontWeight: 'medium', mb: 1}}>
              {t(
                'agents:regenerateSecret.success.securityReminder.title',
                "You won't be able to see this secret again",
              )}
            </Typography>
            <Typography variant="body2">
              {t(
                'agents:regenerateSecret.success.securityReminder.description',
                "Store the new client secret somewhere safe. If you lose it, you'll need to regenerate it again.",
              )}
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
              handleCopy().catch(() => null);
            }}
            disabled={copied}
          >
            {copied
              ? t('agents:regenerateSecret.success.copied', 'Copied')
              : t('agents:regenerateSecret.success.copySecret', 'Copy client secret')}
          </Button>
          <Button variant="outlined" fullWidth onClick={handleClose}>
            {t('common:actions.done', 'Done')}
          </Button>
        </Stack>
      </DialogActions>
    </Dialog>
  );
}
