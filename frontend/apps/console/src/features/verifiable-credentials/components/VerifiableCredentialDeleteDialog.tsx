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

import {Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, Alert} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteVerifiableCredential from '../api/useDeleteVerifiableCredential';

export interface VerifiableCredentialDeleteDialogProps {
  open: boolean;
  vcId: string | null;
  onClose: () => void;
  onSuccess?: () => void;
}

/**
 * Dialog to confirm deletion of a credential configuration.
 */
export default function VerifiableCredentialDeleteDialog({
  open,
  vcId,
  onClose,
  onSuccess = undefined,
}: VerifiableCredentialDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const deleteVC = useDeleteVerifiableCredential();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteVC.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!vcId) return;
    setError(null);
    deleteVC.mutate(vcId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(err.message ?? t('verifiable-credentials:delete.error'));
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('verifiable-credentials:delete.title')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>{t('verifiable-credentials:delete.message')}</DialogContentText>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('verifiable-credentials:delete.disclaimer')}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteVC.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={handleConfirm} color="error" variant="contained" disabled={deleteVC.isPending || !vcId}>
          {deleteVC.isPending ? t('common:status.deleting') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
