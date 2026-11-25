// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for {@link DiscardChangesDialog}.
 *
 * @public
 */
export interface DiscardChangesDialogProps {
  /** Whether the dialog is open. */
  open: boolean;
  /** Called when the user chooses to stay and keep editing. */
  onClose: () => void;
  /** Called when the user confirms discarding unsaved changes. */
  onConfirm: () => void;
  /** What is lost by leaving, naming the thing being edited. Defaults to a generic message. */
  description?: string;
}

/**
 * Confirmation dialog shown before leaving an editor with unsaved changes.
 *
 * @public
 */
export default function DiscardChangesDialog({
  open,
  onClose,
  onConfirm,
  description = undefined,
}: DiscardChangesDialogProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="xs"
      fullWidth
      aria-labelledby="discard-changes-dialog-title"
      aria-describedby="discard-changes-dialog-description"
      data-testid="discard-changes-dialog"
    >
      <DialogTitle id="discard-changes-dialog-title">
        {t('common:discardChanges.title', 'Discard unsaved changes?')}
      </DialogTitle>
      <DialogContent>
        <DialogContentText id="discard-changes-dialog-description">
          {description ??
            t('common:discardChanges.description', 'You have unsaved changes. If you leave now, they will be lost.')}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('common:discardChanges.keepEditing', 'Keep editing')}</Button>
        <Button onClick={onConfirm} color="error" variant="contained" data-testid="discard-changes-confirm-button">
          {t('common:discardChanges.confirm', 'Discard changes')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
