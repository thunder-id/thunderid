// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  List,
  ListItem,
  ListItemText,
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteUser from '../api/useDeleteUser';
import useGetUserUsages from '../api/useGetUserUsages';
import type {UserUsage} from '../models/users';
import getUserErrorMessage from '../utils/getUserErrorMessage';

const MAX_VISIBLE_USAGES = 5;

export interface UserDeleteDialogProps {
  open: boolean;
  userId: string | null;
  onClose: () => void;
  onSuccess?: () => void;
}

/**
 * Dialog component for confirming user deletion.
 */
export default function UserDeleteDialog({
  open,
  userId,
  onClose,
  onSuccess = undefined,
}: UserDeleteDialogProps): JSX.Element {
  const {t} = useTranslation('users');
  const deleteUser = useDeleteUser();
  const [error, setError] = useState<string | null>(null);

  const {data: usagesData, isLoading: isLoadingUsages} = useGetUserUsages(userId, open);

  const usagesKnown = usagesData !== undefined && usagesData.totalResults !== null;
  const blockingUsages = usagesData?.usages.filter((usage) => usage.behaviorOnDelete === 'restrict') ?? [];
  const hasBlockingUsages = usagesKnown && blockingUsages.length > 0;
  const visibleBlocking = blockingUsages.slice(0, MAX_VISIBLE_USAGES);
  const hiddenBlockingCount = blockingUsages.length - visibleBlocking.length;
  // The user's personal agents, which are deleted with the user.
  const cascadingUsages = usagesData?.usages.filter((usage) => usage.behaviorOnDelete === 'cascade') ?? [];
  const visibleCascading = cascadingUsages.slice(0, MAX_VISIBLE_USAGES);
  const hiddenCascadingCount = cascadingUsages.length - visibleCascading.length;

  const renderUsages = (usages: UserUsage[], hiddenCount: number): JSX.Element => (
    <List dense disablePadding>
      {usages.map((usage) => (
        <ListItem key={usage.id} disableGutters sx={{py: 0}}>
          <ListItemText primary={<Typography variant="body2">{usage.displayName}</Typography>} />
        </ListItem>
      ))}
      {hiddenCount > 0 && (
        <ListItem disableGutters sx={{py: 0}}>
          <ListItemText
            primary={
              <Typography variant="body2" color="text.secondary">
                {t('delete.usages.more', {count: hiddenCount, defaultValue: '+{{count}} more'})}
              </Typography>
            }
          />
        </ListItem>
      )}
    </List>
  );

  const handleCancel = (): void => {
    if (deleteUser.isPending) return;
    setError(null);
    deleteUser.reset();
    onClose();
  };

  const handleConfirm = (): void => {
    if (!userId) return;

    setError(null);
    deleteUser.mutate(userId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(getUserErrorMessage(err, t, 'delete.error', 'Failed to delete user. Please try again.'));
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('delete.title', 'Delete user')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t('delete.message', 'Are you sure you want to delete this user? This action cannot be undone.')}
        </DialogContentText>

        {isLoadingUsages ? (
          <Alert severity="info" icon={<CircularProgress size={16} />} sx={{mb: 2}}>
            {t('delete.usages.loading', 'Checking affected resources…')}
          </Alert>
        ) : !usagesKnown ? (
          <Alert severity="warning" sx={{mb: 2}}>
            {t('delete.disclaimer', 'All associated data will be permanently removed.')}
          </Alert>
        ) : hasBlockingUsages ? (
          <Alert severity="error" sx={{mb: 2}}>
            <Typography variant="body2" sx={{mb: 1}}>
              {t(
                'delete.blocking.title',
                'This user cannot be deleted until the following agents are reassigned or removed:',
              )}
            </Typography>
            {renderUsages(visibleBlocking, hiddenBlockingCount)}
          </Alert>
        ) : cascadingUsages.length > 0 ? (
          <Alert severity="warning" sx={{mb: 2}}>
            <Typography variant="body2" sx={{mb: 1}}>
              {t(
                'delete.cascading.title',
                'These personal agents will be deleted with the user, and their tokens revoked:',
              )}
            </Typography>
            {renderUsages(visibleCascading, hiddenCascadingCount)}
          </Alert>
        ) : null}

        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteUser.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={handleConfirm}
          color="error"
          variant="contained"
          disabled={deleteUser.isPending || !userId || isLoadingUsages || hasBlockingUsages}
        >
          {deleteUser.isPending ? t('common:status.deleting', 'Deleting...') : t('common:actions.delete', 'Delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
