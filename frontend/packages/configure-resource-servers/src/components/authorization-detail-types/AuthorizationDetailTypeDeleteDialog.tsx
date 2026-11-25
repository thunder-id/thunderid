// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography} from '@wso2/oxygen-ui';
import {useCallback, useState, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import useDeleteAuthorizationDetailType from '../../api/useDeleteAuthorizationDetailType';
import {errorLogDetails} from '../../hooks/useAuthorizationDetailTypeForm';
import type {AuthorizationDetailType} from '../../models/authorization-detail-type';

export interface AuthorizationDetailTypeDeleteDialogProps {
  detailType: AuthorizationDetailType | null;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function AuthorizationDetailTypeDeleteDialog({
  detailType,
  onClose,
  onSuccess = undefined,
}: AuthorizationDetailTypeDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger('AuthorizationDetailTypeDeleteDialog');
  const deleteType = useDeleteAuthorizationDetailType();
  const [error, setError] = useState<string | null>(null);

  // Resolves an error through the `authorizationDetailTypes` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `authorizationDetailTypes:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `authorizationDetailTypes:${key}`, options),
    [t],
  );

  const handleClose = (): void => {
    if (deleteType.isPending) return;
    setError(null);
    onClose();
  };

  const handleDelete = (): void => {
    if (!detailType) return;
    deleteType.mutate(detailType.id, {
      onSuccess: () => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        logger.error('Failed to delete intent', errorLogDetails(err));
        setError(getErrorMessage(err, tForErrors, 'delete.error', 'Failed to delete the intent. Please try again.'));
      },
    });
  };

  return (
    <Dialog open={detailType !== null} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>{t('authorizationDetailTypes:delete.title', 'Delete intent')}</DialogTitle>
      <DialogContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t(
            'authorizationDetailTypes:delete.warning',
            'Clients can no longer request this intent, and earlier approvals of it no longer apply, so refreshed tokens leave it out. This action cannot be undone.',
          )}
        </Alert>
        <Typography variant="body2">
          <Trans
            i18nKey="authorizationDetailTypes:delete.confirm"
            defaults="Are you sure you want to delete <strong>{{name}}</strong>?"
            values={{name: detailType?.name}}
            components={{strong: <strong />}}
          />
        </Typography>
        {error && (
          <Alert severity="error" sx={{mt: 2}} role="alert">
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={handleClose} disabled={deleteType.isPending}>
          {t('common:cancel', 'Cancel')}
        </Button>
        <Button variant="contained" color="error" onClick={handleDelete} disabled={deleteType.isPending}>
          {deleteType.isPending ? t('common:deleting', 'Deleting…') : t('common:delete', 'Delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
