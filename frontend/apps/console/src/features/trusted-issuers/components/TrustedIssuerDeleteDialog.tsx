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

import {Alert, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteTrustedIssuer from '../api/useDeleteTrustedIssuer';

/**
 * Props for the {@link TrustedIssuerDeleteDialog} component.
 */
export interface TrustedIssuerDeleteDialogProps {
  /**
   * Whether the dialog is open.
   */
  open: boolean;
  /**
   * The id of the trusted issuer to delete.
   */
  trustedIssuerId: string | null;
  /**
   * The display name of the trusted issuer, shown in the confirmation message.
   */
  trustedIssuerName: string;
  /**
   * Called when the dialog should be closed without deleting.
   */
  onClose: () => void;
  /**
   * Called when the trusted issuer is successfully deleted.
   */
  onSuccess?: () => void;
}

/**
 * Confirmation dialog for deleting a trusted issuer.
 */
export default function TrustedIssuerDeleteDialog({
  open,
  trustedIssuerId,
  trustedIssuerName,
  onClose,
  onSuccess = undefined,
}: TrustedIssuerDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const deleteTrustedIssuer = useDeleteTrustedIssuer();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteTrustedIssuer.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!trustedIssuerId) return;

    deleteTrustedIssuer.mutate(trustedIssuerId, {
      onSuccess: (): void => {
        setError(null);
        onClose();
        onSuccess?.();
      },
      onError: (err: Error) => {
        setError(err.message || t('trustedIssuers:delete.error', 'Failed to delete trusted issuer. Please try again.'));
      },
    });
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('trustedIssuers:delete.title', 'Delete trusted issuer')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t(
            'trustedIssuers:delete.message',
            'Delete "{{name}}"? Applications relying on assertions from this issuer will stop receiving tokens. This cannot be undone.',
            {name: trustedIssuerName},
          )}
        </DialogContentText>
        {error && <Alert severity="error">{error}</Alert>}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteTrustedIssuer.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={handleConfirm}
          color="error"
          variant="contained"
          disabled={deleteTrustedIssuer.isPending}
          data-testid="trusted-issuer-delete-confirm"
        >
          {deleteTrustedIssuer.isPending ? t('common:status.deleting') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
