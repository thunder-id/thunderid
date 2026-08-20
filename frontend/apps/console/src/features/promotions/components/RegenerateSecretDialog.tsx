// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Copy} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useRegenerateEnvironmentSecret from '../api/useRegenerateEnvironmentSecret';
import type {RegeneratedSecret, SecretEntry} from '../models/promotion';

interface RegenerateSecretDialogProps {
  open: boolean;
  envId: string;
  secret: SecretEntry | null;
  onClose: () => void;
}

/**
 * Issues a fresh credential and shows it once.
 *
 * The reveal is not a convenience: the Data Plane keeps only a hash, so a value not copied here is gone.
 * Whoever presents the credential has to be given the new one, which is why the confirmation says who
 * that is rather than warning in the abstract.
 */
export default function RegenerateSecretDialog({
  open,
  envId,
  secret,
  onClose,
}: RegenerateSecretDialogProps): JSX.Element {
  const {t} = useTranslation();
  const regenerate = useRegenerateEnvironmentSecret();
  const [issued, setIssued] = useState<string>('');
  const [copied, setCopied] = useState<boolean>(false);

  // Reset as the dialog opens, during render rather than in an effect: an effect would render the
  // previous secret once before clearing it.
  const [wasOpen, setWasOpen] = useState<boolean>(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setIssued('');
      setCopied(false);
    }
  }

  const handleRegenerate = (): void => {
    if (!secret) {
      return;
    }
    regenerate.mutate(
      {envId, name: secret.name},
      {
        onSuccess: (data: RegeneratedSecret) => {
          setIssued(data.value);
        },
      },
    );
  };

  const handleCopy = (): void => {
    navigator.clipboard.writeText(issued).then(
      () => {
        setCopied(true);
      },
      () => {
        // Clipboard access can be refused; the value stays on screen to copy by hand.
      },
    );
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {issued
          ? t('promotions:secrets.regeneratedTitle', 'New secret issued')
          : t('promotions:secrets.regenerateTitle', 'Regenerate secret')}
      </DialogTitle>
      <DialogContent>
        <DialogContentText sx={{wordBreak: 'break-all'}}>{secret?.name}</DialogContentText>

        {!issued && (
          <Alert severity="warning" sx={{mt: 2}}>
            {consequenceOf(secret, t)}
          </Alert>
        )}

        {issued && (
          <>
            <Alert severity="warning" sx={{mt: 2}}>
              {t(
                'promotions:secrets.regeneratedNotice',
                'This value is shown only now. The Data Plane keeps a hash of it, so it cannot be recovered later.',
              )}
            </Alert>
            <Box sx={{mt: 2, p: 2, bgcolor: 'action.hover', borderRadius: 1}}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Typography variant="body2" sx={{fontFamily: 'monospace', wordBreak: 'break-all', flexGrow: 1}}>
                  {issued}
                </Typography>
                <Tooltip
                  title={
                    copied
                      ? t('promotions:secrets.copied', 'Copied')
                      : t('promotions:secrets.copy', 'Copy to clipboard')
                  }
                >
                  <IconButton size="small" onClick={handleCopy}>
                    <Copy size={16} />
                  </IconButton>
                </Tooltip>
              </Stack>
            </Box>
          </>
        )}
      </DialogContent>
      <DialogActions>
        {issued ? (
          <Button variant="contained" onClick={onClose}>
            {t('promotions:secrets.done', 'Done')}
          </Button>
        ) : (
          <>
            <Button onClick={onClose} disabled={regenerate.isPending}>
              {t('common:actions.cancel', 'Cancel')}
            </Button>
            <Button variant="contained" color="warning" onClick={handleRegenerate} disabled={regenerate.isPending}>
              {regenerate.isPending
                ? t('promotions:secrets.regenerating', 'Regenerating...')
                : t('promotions:secrets.regenerate', 'Regenerate')}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}

/**
 * Says who stops working the moment the credential changes.
 *
 * A user's password is the case that catches people out: regenerating one is a password reset, and the
 * person it belongs to is locked out until someone hands them the new value.
 */
function consequenceOf(secret: SecretEntry | null, t: (key: string, fallback: string) => string): string {
  switch (secret?.resourceType) {
    case 'user':
      return t(
        'promotions:secrets.regenerateUserWarning',
        "This resets the user's password on the Data Plane. They cannot sign in until you give them the new one, so copy it from the next screen.",
      );
    case 'agent':
      return t(
        'promotions:secrets.regenerateAgentWarning',
        'The agent stops authenticating until it is configured with the new secret.',
      );
    case 'application':
      return t(
        'promotions:secrets.regenerateApplicationWarning',
        'Every client using this application stops authenticating until it is configured with the new secret.',
      );
    default:
      return t(
        'promotions:secrets.regenerateWarning',
        'Whatever presents this credential stops working until it is given the new value.',
      );
  }
}
