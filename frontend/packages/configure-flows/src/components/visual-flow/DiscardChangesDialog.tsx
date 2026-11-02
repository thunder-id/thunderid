// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface DiscardChangesDialogProps {
  /** Whether the dialog is open. */
  open: boolean;
  /** Called when the user chooses to stay and keep editing. */
  onClose: () => void;
  /** Called when the user confirms discarding unsaved changes. */
  onConfirm: () => void;
}

/**
 * Confirmation dialog shown before leaving the flow builder with unsaved changes.
 */
export default function DiscardChangesDialog({open, onClose, onConfirm}: DiscardChangesDialogProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="discard-changes-dialog-title"
      aria-describedby="discard-changes-dialog-description"
      data-testid="discard-changes-dialog"
    >
      <DialogTitle id="discard-changes-dialog-title">
        {t('flows:core.dialogs.discardChanges.title', 'Discard unsaved changes?')}
      </DialogTitle>
      <DialogContent>
        <DialogContentText id="discard-changes-dialog-description">
          {t(
            'flows:core.dialogs.discardChanges.description',
            'You have unsaved changes to this flow. If you leave now, your changes will be lost.',
          )}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('flows:core.dialogs.discardChanges.cancelButton', 'Keep editing')}</Button>
        <Button onClick={onConfirm} color="error" variant="contained" data-testid="discard-changes-confirm-button">
          {t('flows:core.dialogs.discardChanges.confirmButton', 'Discard changes')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
