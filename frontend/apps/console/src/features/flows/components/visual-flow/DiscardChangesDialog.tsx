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

export interface DiscardChangesDialogProps {
  /** Whether the dialog is open. */
  open: boolean;
  /** Called when the user chooses to stay and keep editing. */
  onClose: () => void;
  /** Called when the user confirms discarding unsaved changes. */
  onConfirm: () => void;
}

/**
 * Confirmation dialog shown before leaving the flow builder with unsaved changes.
 */
export default function DiscardChangesDialog({open, onClose, onConfirm}: DiscardChangesDialogProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth data-testid="discard-changes-dialog">
      <DialogTitle>{t('flows:core.dialogs.discardChanges.title', 'Discard unsaved changes?')}</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {t(
            'flows:core.dialogs.discardChanges.description',
            'You have unsaved changes to this flow. If you leave now, your changes will be lost.',
          )}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('flows:core.dialogs.discardChanges.cancelButton', 'Keep editing')}</Button>
        <Button onClick={onConfirm} color="error" variant="contained" data-testid="discard-changes-confirm-button">
          {t('flows:core.dialogs.discardChanges.confirmButton', 'Discard changes')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
