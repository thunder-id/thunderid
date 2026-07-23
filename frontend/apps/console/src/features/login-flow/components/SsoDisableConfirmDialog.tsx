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

import {Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface SsoDisableConfirmDialogProps {
  /**
   * Whether the dialog is open.
   */
  open: boolean;
  /**
   * Number of SSO checkpoints that will be removed.
   */
  checkpointCount: number;
  /**
   * Callback when the dialog should be closed without removing anything.
   */
  onClose: () => void;
  /**
   * Callback when the user confirms the removal.
   */
  onConfirm: () => void;
}

/**
 * Confirmation dialog shown before removing the SSO wiring from a login flow.
 */
export default function SsoDisableConfirmDialog({
  open,
  checkpointCount,
  onClose,
  onConfirm,
}: SsoDisableConfirmDialogProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('flows:sso.confirmDialog.title', 'Remove single sign-on?')}</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {t('flows:sso.confirmDialog.description', {
            count: checkpointCount,
            defaultValue_one:
              'This removes {{count}} SSO checkpoint and its session step, and reconnects the flow. Users will authenticate with their credentials every time.',
            defaultValue_other:
              'This removes {{count}} SSO checkpoints and their session steps, and reconnects the flow. Users will authenticate with their credentials every time.',
          })}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('flows:sso.confirmDialog.cancelButton', 'Cancel')}</Button>
        <Button onClick={onConfirm} color="error" variant="contained" data-testid="sso-disable-confirm-button">
          {t('flows:sso.confirmDialog.confirmButton', 'Remove SSO')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
