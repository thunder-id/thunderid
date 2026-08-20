// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import useDeleteEnvironmentVariable from '../api/useDeleteEnvironmentVariable';

interface EnvironmentVariableDeleteDialogProps {
  open: boolean;
  environmentVariableId: string;
  onClose: () => void;
}

/**
 * Confirms deleting an environment variable.
 */
export default function EnvironmentVariableDeleteDialog({
  open,
  environmentVariableId,
  onClose,
}: EnvironmentVariableDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const {envId = ''} = useParams<{envId: string}>();
  const deleteEnvironmentVariable = useDeleteEnvironmentVariable(envId);

  const handleDelete = (): void => {
    deleteEnvironmentVariable.mutate(environmentVariableId, {onSuccess: onClose});
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('environmentVariables:delete.title', 'Delete Environment Variable')}</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {t(
            'environmentVariables:delete.message',
            'Are you sure you want to delete this environment variable? This action cannot be undone.',
          )}
        </DialogContentText>
        <Alert severity="warning" sx={{mt: 2}}>
          {t(
            'environmentVariables:delete.disclaimer',
            'Configuration that references this variable will no longer resolve when applied to a Data Plane.',
          )}
        </Alert>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={deleteEnvironmentVariable.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" color="error" onClick={handleDelete} disabled={deleteEnvironmentVariable.isPending}>
          {deleteEnvironmentVariable.isPending
            ? t('common:status.deleting', 'Deleting...')
            : t('common:actions.delete', 'Delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
