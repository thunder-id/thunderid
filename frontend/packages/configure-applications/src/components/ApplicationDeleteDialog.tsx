// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteApplication from '../api/useDeleteApplication';
import getApplicationErrorMessage from '../utils/getApplicationErrorMessage';

export interface ApplicationDeleteDialogProps {
  /**
   * Whether the dialog is open
   */
  open: boolean;
  /**
   * The ID of the application to delete
   */
  applicationId: string | null;
  /**
   * Callback when the dialog should be closed
   */
  onClose: () => void;
  /**
   * Callback when the application is successfully deleted
   */
  onSuccess?: () => void;
}

/**
 * Dialog component for confirming application deletion
 */
export default function ApplicationDeleteDialog({
  open,
  applicationId,
  onClose,
  onSuccess = undefined,
}: ApplicationDeleteDialogProps): JSX.Element {
  const {t} = useTranslation('applications');
  const deleteApplication = useDeleteApplication();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteApplication.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!applicationId) return;

    setError(null);
    deleteApplication.mutate(applicationId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(getApplicationErrorMessage(err, t, 'delete.error', 'Failed to delete application. Please try again.'));
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('delete.title', 'Delete Application')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t('delete.message', 'Are you sure you want to delete this application? This action cannot be undone.')}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t(
            'delete.disclaimer',
            'Warning: All associated data, configurations, and access tokens will be permanently removed.',
          )}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteApplication.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={handleConfirm} color="error" variant="contained" disabled={deleteApplication.isPending}>
          {deleteApplication.isPending ? t('common:status.deleting') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
