// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger';
import {getErrorMessage} from '@thunderid/utils';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useRegenerateAgentSecret from '../api/useRegenerateAgentSecret';

export interface RegenerateSecretDialogProps {
  open: boolean;
  agentId: string | null;
  onClose: () => void;
  onSuccess?: (newClientSecret: string) => void;
  onError?: (message: string) => void;
}

export default function RegenerateSecretDialog({
  open,
  agentId,
  onClose,
  onSuccess = undefined,
  onError = undefined,
}: RegenerateSecretDialogProps): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger('RegenerateSecretDialog');
  const [error, setError] = useState<string | null>(null);
  const regenerateClientSecret = useRegenerateAgentSecret();

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  const handleCancel = (): void => {
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!agentId) {
      setError(t('agents:regenerateSecret.dialog.error', 'Failed to regenerate client secret'));
      return;
    }

    setError(null);
    logger.info('Regenerating agent client secret', {agentId});

    regenerateClientSecret.mutate(
      {agentId},
      {
        onSuccess: ({clientSecret}) => {
          logger.info('Agent client secret regenerated successfully.', {agentId});
          onClose();
          onSuccess?.(clientSecret);
        },
        onError: (err) => {
          const errorMessage = getErrorMessage(
            err,
            tForErrors,
            'regenerateSecret.dialog.error',
            'Failed to regenerate client secret',
          );
          logger.error('Failed to regenerate agent client secret', {
            agentId,
            errorMessage,
            errorName: err instanceof Error ? err.name : 'UnknownError',
          });
          setError(errorMessage);
          onError?.(errorMessage);
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('agents:regenerateSecret.dialog.title', 'Regenerate client secret?')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t(
            'agents:regenerateSecret.dialog.message',
            'A new client secret will be generated for this agent. Any service using the current client secret will stop working immediately.',
          )}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t(
            'agents:regenerateSecret.dialog.disclaimer',
            'This action cannot be undone. The current client secret will be invalidated as soon as you confirm.',
          )}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={regenerateClientSecret.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={handleConfirm}
          color="error"
          variant="contained"
          disabled={regenerateClientSecret.isPending || !agentId}
        >
          {regenerateClientSecret.isPending
            ? t('agents:regenerateSecret.dialog.regenerating', 'Regenerating…')
            : t('agents:regenerateSecret.dialog.confirmButton', 'Regenerate')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
