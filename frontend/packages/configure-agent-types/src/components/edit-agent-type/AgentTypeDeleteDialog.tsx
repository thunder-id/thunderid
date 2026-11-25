// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteAgentType from '../../api/useDeleteAgentType';

/**
 * Props for the {@link AgentTypeDeleteDialog} component.
 *
 * @public
 */
export interface AgentTypeDeleteDialogProps {
  /**
   * Whether the dialog is open.
   */
  open: boolean;
  /**
   * The ID of the agent type to delete, or null when none is selected.
   */
  agentTypeId: string | null;
  /**
   * Invoked when the dialog closes, after a cancel or a successful delete.
   */
  onClose: () => void;
  /**
   * Invoked after the agent type is deleted.
   */
  onSuccess?: () => void;
}

/**
 * Dialog component for confirming agent type deletion. A failed delete keeps the dialog open
 * and shows the reason inside it.
 *
 * @public
 */
export default function AgentTypeDeleteDialog({
  open,
  agentTypeId,
  onClose,
  onSuccess = undefined,
}: AgentTypeDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const deleteAgentType = useDeleteAgentType();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteAgentType.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!agentTypeId) return;

    setError(null);
    deleteAgentType.mutate(agentTypeId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(
          getErrorMessage(
            err,
            (key, options) => t(key.includes(':') ? key : `agentTypes:${key}`, options),
            'delete.error',
            'Failed to delete agent type. Please try again.',
          ),
        );
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('agentTypes:delete.title', 'Delete agent type')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t(
            'agentTypes:delete.message',
            'Are you sure you want to delete this agent type? This action cannot be undone.',
          )}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('agentTypes:delete.disclaimer', 'All associated schema definitions will be permanently removed.')}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteAgentType.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          onClick={handleConfirm}
          color="error"
          variant="contained"
          disabled={deleteAgentType.isPending || !agentTypeId}
        >
          {deleteAgentType.isPending
            ? t('common:status.deleting', 'Deleting...')
            : t('common:actions.delete', 'Delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
