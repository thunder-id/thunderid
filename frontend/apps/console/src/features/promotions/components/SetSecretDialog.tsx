// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useSetEnvironmentSecret from '../api/useSetEnvironmentSecret';
import type {SecretEntry} from '../models/promotion';

interface SetSecretDialogProps {
  open: boolean;
  envId: string;
  secret: SecretEntry | null;
  onClose: () => void;
}

/**
 * Sets one credential to a value the operator supplies.
 *
 * This is the only way to fill a credential the Data Plane replays to a third party, because that value
 * is issued elsewhere and cannot be generated here.
 */
export default function SetSecretDialog({open, envId, secret, onClose}: SetSecretDialogProps): JSX.Element {
  const {t} = useTranslation();
  const setSecret = useSetEnvironmentSecret();
  const [value, setValue] = useState<string>('');

  // Reset as the dialog opens, during render rather than in an effect: an effect would render the
  // previous value once before clearing it.
  const [wasOpen, setWasOpen] = useState<boolean>(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setValue('');
    }
  }

  const handleSave = (): void => {
    if (!secret || value === '') {
      return;
    }
    setSecret.mutate({envId, name: secret.name, value}, {onSuccess: () => onClose()});
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('promotions:secrets.setTitle', 'Set secret')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{wordBreak: 'break-all'}}>{secret?.name}</DialogContentText>
        {secret?.kind === 'hash' ? (
          <Alert severity="info" sx={{mt: 2}}>
            {t(
              'promotions:secrets.setHashNotice',
              'This credential is stored as a one-way hash, so it cannot be read back afterwards. Keep a copy of what you enter.',
            )}
          </Alert>
        ) : (
          <Alert severity="info" sx={{mt: 2}}>
            {t(
              'promotions:secrets.setValueNotice',
              'This credential is replayed to an external service, so enter exactly the value that service issued.',
            )}
          </Alert>
        )}
        <TextField
          fullWidth
          sx={{mt: 2}}
          type="password"
          autoComplete="new-password"
          label={t('promotions:secrets.valueLabel', 'Value')}
          value={value}
          onChange={(event) => {
            setValue(event.target.value);
          }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={setSecret.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" onClick={handleSave} disabled={setSecret.isPending || value === ''}>
          {setSecret.isPending ? t('promotions:secrets.saving', 'Saving...') : t('common:actions.save', 'Save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
