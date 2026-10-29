// Copyright 2025 The ThunderID Authors
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
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetConnectionUsages from '../api/useGetConnectionUsages';
import type {ConnectionType} from '../models/connection';

const MAX_VISIBLE_USAGES = 5;

interface ConnectionDeleteDialogProps {
  open: boolean;
  connectionType: ConnectionType;
  connectionId: string;
  connectionName: string;
  /** Whether this connection is the default email sender, which server-sent email goes through. */
  isDefaultEmailSender?: boolean;
  isPending: boolean;
  /** Resolved error message from a failed delete, rendered inline. The dialog stays open on failure. */
  error?: string | null;
  onConfirm: () => void;
  onClose: () => void;
}

export default function ConnectionDeleteDialog({
  open,
  connectionType,
  connectionId,
  connectionName,
  isDefaultEmailSender = false,
  isPending,
  error = null,
  onConfirm,
  onClose,
}: ConnectionDeleteDialogProps): JSX.Element {
  const {t} = useTranslation('connections');

  const {data: usagesData, isLoading: isLoadingUsages} = useGetConnectionUsages(
    connectionType,
    connectionId || undefined,
    open,
  );

  const usagesKnown = usagesData !== undefined && usagesData.totalResults !== null;
  const blockingUsages = usagesData?.usages.filter((usage) => usage.behaviorOnDelete === 'restrict') ?? [];
  const hasBlockingUsages = usagesKnown && blockingUsages.length > 0;
  const visibleBlocking = blockingUsages.slice(0, MAX_VISIBLE_USAGES);
  const hiddenBlockingCount = blockingUsages.length - visibleBlocking.length;

  return (
    <Dialog open={open} onClose={isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('delete.title')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>{t('delete.message', {name: connectionName})}</DialogContentText>

        {isDefaultEmailSender && (
          <Alert severity="warning" sx={{mb: 2}} data-testid="connection-delete-default-sender-warning">
            {t(
              'delete.defaultEmailSender',
              'This is the default email sender. After you delete it, system-generated emails, such as account lock notifications, are not sent until another sender is set as the default.',
            )}
          </Alert>
        )}

        {isLoadingUsages ? (
          <Alert severity="info" icon={<CircularProgress size={16} />}>
            {t('delete.usages.loading')}
          </Alert>
        ) : hasBlockingUsages ? (
          <Alert severity="error">
            <Typography variant="body2" sx={{mb: 1}}>
              {t('delete.blocking.title')}
            </Typography>
            <List dense disablePadding>
              {visibleBlocking.map((usage) => (
                <ListItem key={usage.id} disableGutters sx={{py: 0}}>
                  <ListItemText primary={<Typography variant="body2">{usage.displayName}</Typography>} />
                </ListItem>
              ))}
              {hiddenBlockingCount > 0 && (
                <ListItem disableGutters sx={{py: 0}}>
                  <ListItemText
                    primary={
                      <Typography variant="body2" color="text.secondary">
                        {t('delete.usages.more', {count: hiddenBlockingCount})}
                      </Typography>
                    }
                  />
                </ListItem>
              )}
            </List>
          </Alert>
        ) : null}

        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={onConfirm}
          color="error"
          variant="contained"
          disabled={isPending || isLoadingUsages || hasBlockingUsages}
          data-testid="connection-delete-confirm"
        >
          {t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
