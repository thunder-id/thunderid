// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteUserType from '../../api/useDeleteUserType';
import useGetUserTypeUsages from '../../api/useGetUserTypeUsages';

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

  const {data: usagesData, isLoading: isLoadingUsages} = useGetUserTypeUsages(userTypeId, open);

  const usagesKnown = usagesData !== undefined && usagesData.totalResults !== null;
  const blockingUsageCount =
    usagesData?.usages.filter((usage) => usage.behaviorOnDelete === 'restrict').length ?? 0;
  const hasBlockingUsages = usagesKnown && blockingUsageCount > 0;

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
      <DialogTitle>{t('userTypes:delete.title', 'Delete User Type')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t(
            'userTypes:delete.message',
            'Are you sure you want to delete this user type? This action cannot be undone.',
          )}
        </DialogContentText>

        {isLoadingUsages ? (
          <Alert severity="info" icon={<CircularProgress size={16} />} sx={{mb: 2}}>
            {t('userTypes:delete.usages.loading', 'Checking affected resources…')}
          </Alert>
        ) : !usagesKnown ? (
          <Alert severity="warning" sx={{mb: 2}}>
            {t('userTypes:delete.disclaimer', 'All associated schema definitions will be permanently removed.')}
          </Alert>
        ) : hasBlockingUsages ? (
          <Alert severity="error" sx={{mb: 2}}>
            {t(
              'userTypes:delete.blocking.title',
              'This user type cannot be deleted because {{count}} existing user(s) are still assigned to it. Reassign or delete those users first.',
              {count: blockingUsageCount},
            )}
          </Alert>
        ) : null}

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
          disabled={deleteUserType.isPending || !userTypeId || isLoadingUsages || hasBlockingUsages}
        >
          {deleteUserType.isPending ? t('common:status.deleting', 'Deleting...') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
