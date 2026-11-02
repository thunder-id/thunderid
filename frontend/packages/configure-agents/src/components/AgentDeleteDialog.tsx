// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteAgent from '../api/useDeleteAgent';

export interface AgentDeleteDialogProps {
  open: boolean;
  agentId: string | null;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function AgentDeleteDialog({
  open,
  agentId,
  onClose,
  onSuccess = undefined,
}: AgentDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const deleteAgent = useDeleteAgent();
  const [error, setError] = useState<string | null>(null);

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  const handleCancel = (): void => {
    if (deleteAgent.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!agentId) return;

    deleteAgent.mutate(agentId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(getErrorMessage(err, tForErrors, 'delete.error', 'Failed to delete agent. Please try again.'));
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('agents:delete.title', 'Delete agent')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t('agents:delete.message', 'Are you sure you want to delete this agent? This action cannot be undone.')}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('agents:delete.disclaimer', 'Deleting this agent will revoke all its credentials and access tokens.')}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteAgent.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={handleConfirm} color="error" variant="contained" disabled={deleteAgent.isPending}>
          {deleteAgent.isPending ? t('common:status.deleting') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
