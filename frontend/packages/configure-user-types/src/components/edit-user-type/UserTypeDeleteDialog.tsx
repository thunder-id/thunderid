// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteUserType from '../../api/useDeleteUserType';

export interface UserTypeDeleteDialogProps {
  open: boolean;
  userTypeId: string | null;
  onClose: () => void;
  onSuccess?: () => void;
}

/**
 * Dialog component for confirming user type deletion.
 */
export default function UserTypeDeleteDialog({
  open,
  userTypeId,
  onClose,
  onSuccess = undefined,
}: UserTypeDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const deleteUserType = useDeleteUserType();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteUserType.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!userTypeId) return;

    setError(null);
    deleteUserType.mutate(userTypeId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(
          getErrorMessage(
            err,
            (key, options) => t(key.includes(':') ? key : `userTypes:${key}`, options),
            'delete.error',
            'Failed to delete user type. Please try again.',
          ),
        );
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('userTypes:delete.title', 'Delete user type')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t(
            'userTypes:delete.message',
            'Are you sure you want to delete this user type? This action cannot be undone and may affect existing users of this type.',
          )}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('userTypes:delete.disclaimer', 'All associated schema definitions will be permanently removed.')}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteUserType.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={handleConfirm}
          color="error"
          variant="contained"
          disabled={deleteUserType.isPending || !userTypeId}
        >
          {deleteUserType.isPending ? t('common:status.deleting', 'Deleting...') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
