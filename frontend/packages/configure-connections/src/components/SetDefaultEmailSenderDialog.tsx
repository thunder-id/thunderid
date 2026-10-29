// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import useSetDefaultEmailSender from '../api/useSetDefaultEmailSender';

export interface SetDefaultEmailSenderDialogProps {
  open: boolean;
  senderId: string;
  senderName: string;
  /** Whether another sender is the default now, which this one replaces. */
  replacesDefault: boolean;
  onClose: () => void;
}

export default function SetDefaultEmailSenderDialog({
  open,
  senderId,
  senderName,
  replacesDefault,
  onClose,
}: SetDefaultEmailSenderDialogProps): JSX.Element {
  const {t} = useTranslation('connections');
  const {showToast} = useToast();
  const setDefault = useSetDefaultEmailSender();
  const [error, setError] = useState<string | null>(null);

  const handleClose = (): void => {
    if (setDefault.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    setError(null);
    setDefault.mutate(
      {defaultEmailSenderId: senderId},
      {
        onSuccess: () => {
          showToast(
            t('setDefault.success', '{{name}} is now the default email sender.', {name: senderName}),
            'success',
          );
          onClose();
        },
        onError: (err: Error) => {
          setError(getErrorMessage(err, t, 'setDefault.error', 'Failed to set the default email sender.'));
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>{t('setDefault.title', 'Set default email sender')}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          <Trans
            i18nKey="connections:setDefault.message"
            defaults="<bold>{{name}}</bold> will be used as the default sender for system-generated emails, such as account lock notifications."
            values={{name: senderName}}
            components={{bold: <strong />}}
          />
        </Typography>
        {replacesDefault && (
          <Typography variant="body2" color="text.secondary" sx={{mt: 1}}>
            {t('setDefault.replaces', 'This will replace the current default email sender.')}
          </Typography>
        )}
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={handleClose} disabled={setDefault.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          onClick={handleConfirm}
          disabled={setDefault.isPending}
          data-testid="set-default-email-sender-confirm"
        >
          {setDefault.isPending ? t('setDefault.setting', 'Setting…') : t('setDefault.confirm', 'Set as default')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
