// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useRegenerateClientSecret from '../api/useRegenerateClientSecret';
import getApplicationErrorMessage from '../utils/getApplicationErrorMessage';

const DEFAULT_REGENERATE_SECRET_ERROR = 'Failed to regenerate client secret. Please try again.';

/**
 * Props for the {@link RegenerateSecretDialog} component.
 */
export interface RegenerateSecretDialogProps {
  /**
   * Whether the dialog is open
   */
  open: boolean;
  /**
   * The ID of the application whose client secret will be regenerated
   */
  applicationId: string | null;
  /**
   * Callback when the dialog should be closed
   */
  onClose: () => void;
  /**
   * Callback when the client secret is successfully regenerated with the new client secret
   */
  onSuccess?: (newClientSecret: string) => void;
  /**
   * Callback when the regeneration fails
   */
  onError?: (message: string) => void;
}

/**
 * Dialog component for confirming client secret regeneration.
 *
 * This dialog warns users about the consequences of regenerating an application's
 * client secret before proceeding with the action.
 *
 * @param props - Component props
 * @returns The regenerate client secret confirmation dialog
 */
export default function RegenerateSecretDialog({
  open,
  applicationId,
  onClose,
  onSuccess = undefined,
  onError = undefined,
}: RegenerateSecretDialogProps): JSX.Element {
  const {t} = useTranslation('applications');
  const logger = useLogger('RegenerateSecretDialog');
  const [error, setError] = useState<string | null>(null);
  const regenerateClientSecret = useRegenerateClientSecret();

  const handleCancel = (): void => {
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!applicationId) {
      setError(t('regenerateSecret.dialog.error', DEFAULT_REGENERATE_SECRET_ERROR));
      return;
    }

    setError(null);
    logger.info('Regenerating application client secret', {applicationId});

    regenerateClientSecret.mutate(
      {applicationId},
      {
        onSuccess: ({clientSecret}) => {
          logger.info('Application client secret regenerated successfully. New client secret generated.', {
            applicationId,
          });
          onClose();
          onSuccess?.(clientSecret);
        },
        onError: (err) => {
          const errorMessage = getApplicationErrorMessage(
            err,
            t,
            'regenerateSecret.dialog.error',
            DEFAULT_REGENERATE_SECRET_ERROR,
          );
          logger.error('Failed to regenerate client secret', {
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
      <DialogTitle>{t('regenerateSecret.dialog.title', 'Regenerate Client Secret')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t(
            'regenerateSecret.dialog.message',
            'Are you sure you want to regenerate the client secret for this application? This will immediately invalidate the current client secret and generate a new one.',
          )}
        </DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t(
            'regenerateSecret.dialog.disclaimer',
            'Warning: Regenerating the client secret will invalidate the current secret and the application may stop working until the new client secret is updated in its configuration.',
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
          disabled={regenerateClientSecret.isPending || !applicationId}
        >
          {regenerateClientSecret.isPending
            ? t('regenerateSecret.dialog.regenerating', 'Regenerating...')
            : t('regenerateSecret.dialog.confirmButton', 'Regenerate')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
