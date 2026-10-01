// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger';
import {getErrorMessage} from '@thunderid/utils';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useRegenerateFlowSecret from '../api/useRegenerateFlowSecret';

const DEFAULT_REGENERATE_FLOW_SECRET_ERROR = 'Failed to regenerate Flow Secret. Please try again.';

/**
 * Props for the {@link RegenerateFlowSecretDialog} component.
 */
export interface RegenerateFlowSecretDialogProps {
  /**
   * Whether the dialog is open
   */
  open: boolean;
  /**
   * The ID of the application whose Flow Secret will be regenerated
   */
  applicationId: string | null;
  /**
   * Callback when the dialog should be closed
   */
  onClose: () => void;
  /**
   * Callback when the Flow Secret is successfully regenerated with the new Flow Secret
   */
  onSuccess?: (newFlowSecret: string) => void;
  /**
   * Callback when the regeneration fails
   */
  onError?: (message: string) => void;
}

/**
 * Dialog component for confirming Flow Secret regeneration.
 *
 * Warns users that regenerating the Flow Secret immediately invalidates the current one, which will
 * break any server-side flow initiation until the new secret is deployed.
 *
 * @param props - Component props
 * @returns The regenerate Flow Secret confirmation dialog
 */
export default function RegenerateFlowSecretDialog({
  open,
  applicationId,
  onClose,
  onSuccess = undefined,
  onError = undefined,
}: RegenerateFlowSecretDialogProps): JSX.Element {
  const {t} = useTranslation('applications');
  const logger = useLogger('RegenerateFlowSecretDialog');
  const [error, setError] = useState<string | null>(null);
  const regenerateFlowSecret = useRegenerateFlowSecret();

  const handleCancel = (): void => {
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!applicationId) {
      setError(t('regenerateFlowSecret.dialog.error', DEFAULT_REGENERATE_FLOW_SECRET_ERROR));
      return;
    }

    setError(null);
    logger.info('Regenerating application Flow Secret', {applicationId});

    regenerateFlowSecret.mutate(
      {applicationId},
      {
        onSuccess: ({flowSecret}) => {
          logger.info('Application Flow Secret regenerated successfully.', {applicationId});
          onClose();
          onSuccess?.(flowSecret);
        },
        onError: (err) => {
          const errorMessage = getErrorMessage(
            err,
            t,
            'regenerateFlowSecret.dialog.error',
            DEFAULT_REGENERATE_FLOW_SECRET_ERROR,
          );
          logger.error('Failed to regenerate Flow Secret', {
            applicationId,
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
      <DialogTitle>{t('regenerateFlowSecret.dialog.title', 'Regenerate Flow Secret')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t(
            'regenerateFlowSecret.dialog.message',
            'Are you sure you want to regenerate the Flow Secret for this application? This will immediately invalidate the current Flow Secret and generate a new one.',
          )}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t(
            'regenerateFlowSecret.dialog.disclaimer',
            'Warning: Regenerating the Flow Secret invalidates the current secret. Server-side flow initiation will fail until the new Flow Secret is deployed.',
          )}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={regenerateFlowSecret.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={handleConfirm}
          color="error"
          variant="contained"
          disabled={regenerateFlowSecret.isPending || !applicationId}
        >
          {regenerateFlowSecret.isPending
            ? t('regenerateFlowSecret.dialog.regenerating', 'Regenerating...')
            : t('regenerateFlowSecret.dialog.confirmButton', 'Regenerate')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
