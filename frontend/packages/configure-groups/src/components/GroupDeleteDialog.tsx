// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteGroup from '../api/useDeleteGroup';

export interface GroupDeleteDialogProps {
  open: boolean;
  groupId: string | null;
  onClose: () => void;
  onSuccess?: () => void;
}

/**
 * Dialog component for confirming group deletion.
 */
export default function GroupDeleteDialog({
  open,
  groupId,
  onClose,
  onSuccess = undefined,
}: GroupDeleteDialogProps): JSX.Element {
  const {t} = useTranslation('groups');
  const deleteGroup = useDeleteGroup();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteGroup.isPending) return;
    setError(null);
    deleteGroup.reset();
    onClose();
  };

  const handleConfirm = (): void => {
    if (!groupId) return;

    setError(null);
    deleteGroup.mutate(groupId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(getErrorMessage(err, t, 'delete.error', 'Failed to delete group. Please try again.'));
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('delete.title', 'Delete group')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t('delete.message', 'Are you sure you want to delete this group?')}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('delete.disclaimer', 'This action cannot be undone. All group associations will be permanently removed.')}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteGroup.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={handleConfirm} color="error" variant="contained" disabled={deleteGroup.isPending || !groupId}>
          {deleteGroup.isPending ? t('common:status.deleting') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
