// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface GatewayValueDeleteDialogProps {
  title: string;
  message: string;
  isPending: boolean;
  /** The resolved message of a failed delete, shown beside the confirm control. */
  error?: string;
  onConfirm: () => void;
  onClose: () => void;
}

/**
 * Confirms deleting a variable or a secret from a gateway.
 */
export default function GatewayValueDeleteDialog({
  title,
  message,
  isPending,
  error = undefined,
  onConfirm,
  onClose,
}: GatewayValueDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();

  const handleClose = (): void => {
    if (isPending) return;
    onClose();
  };

  return (
    <Dialog open onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Typography variant="body2">{message}</Typography>
        <Alert severity="warning" sx={{mt: 2}}>
          {t(
            'gateways:values.delete.disclaimer',
            'An apply or a revert to a version that refers to it is refused until it is set again.',
          )}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={handleClose} disabled={isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" color="error" onClick={onConfirm} disabled={isPending}>
          {isPending
            ? t('gateways:values.delete.submitting', 'Deleting...')
            : t('gateways:values.delete.submit', 'Delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
