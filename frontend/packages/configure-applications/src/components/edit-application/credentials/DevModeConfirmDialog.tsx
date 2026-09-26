// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface DevModeConfirmDialogProps {
  /**
   * Whether the dialog is open.
   */
  open: boolean;
  /**
   * Callback when the dialog should be closed without enabling dev mode.
   */
  onClose: () => void;
  /**
   * Callback when the user confirms enabling dev mode.
   */
  onConfirm: () => void;
}

/**
 * Confirmation dialog shown before enabling attestation dev mode on a mobile application.
 */
export default function DevModeConfirmDialog({open, onClose, onConfirm}: DevModeConfirmDialogProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {t('applications:edit.advanced.attestation.devModeConfirmDialog.title', 'Enable Dev Mode?')}
      </DialogTitle>
      <DialogContent>
        <DialogContentText>
          {t(
            'applications:edit.advanced.attestation.devModeConfirmDialog.description',
            'This skips attestation verification for this application, so it can initiate a sign-in flow ' +
              'without presenting an attestation token. Use it only for testing, or to try out a sample or ' +
              'development client. Do not enable it in production.',
          )}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>
          {t('applications:edit.advanced.attestation.devModeConfirmDialog.cancelButton', 'Cancel')}
        </Button>
        <Button onClick={onConfirm} color="warning" variant="contained" data-testid="dev-mode-confirm-button">
          {t('applications:edit.advanced.attestation.devModeConfirmDialog.confirmButton', 'Enable Dev Mode')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
