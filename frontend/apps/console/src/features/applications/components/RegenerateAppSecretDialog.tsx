/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useLogger} from '@thunderid/logger';
import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useRegenerateAppSecret from '../api/useRegenerateAppSecret';

/**
 * Props for the {@link RegenerateAppSecretDialog} component.
 */
export interface RegenerateAppSecretDialogProps {
  /**
   * Whether the dialog is open
   */
  open: boolean;
  /**
   * The ID of the application whose App Secret will be regenerated
   */
  applicationId: string | null;
  /**
   * Callback when the dialog should be closed
   */
  onClose: () => void;
  /**
   * Callback when the App Secret is successfully regenerated with the new App Secret
   */
  onSuccess?: (newAppSecret: string) => void;
  /**
   * Callback when the regeneration fails
   */
  onError?: (message: string) => void;
}

/**
 * Dialog component for confirming App Secret regeneration.
 *
 * Warns users that regenerating the App Secret immediately invalidates the current one, which will
 * break any server-side flow initiation until the new secret is deployed.
 *
 * @param props - Component props
 * @returns The regenerate App Secret confirmation dialog
 */
export default function RegenerateAppSecretDialog({
  open,
  applicationId,
  onClose,
  onSuccess = undefined,
  onError = undefined,
}: RegenerateAppSecretDialogProps): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger('RegenerateAppSecretDialog');
  const [error, setError] = useState<string | null>(null);
  const regenerateAppSecret = useRegenerateAppSecret();

  const handleCancel = (): void => {
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!applicationId) {
      setError(t('applications:regenerateAppSecret.dialog.error'));
      return;
    }

    setError(null);
    logger.info('Regenerating application App Secret', {applicationId});

    regenerateAppSecret.mutate(
      {applicationId},
      {
        onSuccess: ({appSecret}) => {
          logger.info('Application App Secret regenerated successfully.', {applicationId});
          onClose();
          onSuccess?.(appSecret);
        },
        onError: (err) => {
          const errorMessage = err instanceof Error ? err.message : t('applications:regenerateAppSecret.dialog.error');
          logger.error('Failed to regenerate App Secret', {
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
      <DialogTitle>{t('applications:regenerateAppSecret.dialog.title')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>{t('applications:regenerateAppSecret.dialog.message')}</DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('applications:regenerateAppSecret.dialog.disclaimer')}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={regenerateAppSecret.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={handleConfirm}
          color="error"
          variant="contained"
          disabled={regenerateAppSecret.isPending || !applicationId}
        >
          {regenerateAppSecret.isPending
            ? t('applications:regenerateAppSecret.dialog.regenerating')
            : t('applications:regenerateAppSecret.dialog.confirmButton')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
