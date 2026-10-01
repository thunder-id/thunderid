// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface SsoDisableConfirmDialogProps {
  /**
   * Whether the dialog is open.
   */
  open: boolean;
  /**
   * Number of SSO checkpoints that will be removed.
   */
  checkpointCount: number;
  /**
   * Callback when the dialog should be closed without removing anything.
   */
  onClose: () => void;
  /**
   * Callback when the user confirms the removal.
   */
  onConfirm: () => void;
}

/**
 * Confirmation dialog shown before removing the SSO wiring from a login flow.
 */
export default function SsoDisableConfirmDialog({
  open,
  checkpointCount,
  onClose,
  onConfirm,
}: SsoDisableConfirmDialogProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('flows:sso.confirmDialog.title', 'Remove single sign-on?')}</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {t('flows:sso.confirmDialog.description', {
            count: checkpointCount,
            defaultValue_one:
              'This removes {{count}} SSO checkpoint and its session step, and reconnects the flow. Users will authenticate with their credentials every time.',
            defaultValue_other:
              'This removes {{count}} SSO checkpoints and their session steps, and reconnects the flow. Users will authenticate with their credentials every time.',
          })}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('flows:sso.confirmDialog.cancelButton', 'Cancel')}</Button>
        <Button onClick={onConfirm} color="error" variant="contained" data-testid="sso-disable-confirm-button">
          {t('flows:sso.confirmDialog.confirmButton', 'Remove SSO')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
