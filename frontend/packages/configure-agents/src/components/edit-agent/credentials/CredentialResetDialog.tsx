// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CredentialFieldInput} from '@thunderid/configure-users';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  FormLabel,
  Stack,
} from '@wso2/oxygen-ui';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useUpdateAgentCredentials from '../../../api/useUpdateAgentCredentials';

export interface AgentCredentialField {
  fieldName: string;
  label: string;
}

interface CredentialResetDialogProps {
  open: boolean;
  field: AgentCredentialField | null;
  agentId: string;
  onClose: () => void;
}

export default function CredentialResetDialog({
  open,
  field,
  agentId,
  onClose,
}: CredentialResetDialogProps): JSX.Element {
  const {t} = useTranslation();
  const updateCredentials = useUpdateAgentCredentials();

  const [newValue, setNewValue] = useState('');
  const [confirmValue, setConfirmValue] = useState('');
  const [mismatch, setMismatch] = useState(false);

  // Resolves an error through the `agents` catalog, as the secret regeneration dialog does.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  const handleClose = (): void => {
    setNewValue('');
    setConfirmValue('');
    setMismatch(false);
    updateCredentials.reset();
    onClose();
  };

  const handleSave = (): void => {
    if (!field || newValue.trim() === '') return;

    if (newValue !== confirmValue) {
      setMismatch(true);
      return;
    }

    updateCredentials.mutate({agentId, data: {credentials: {[field.fieldName]: newValue}}}, {onSuccess: handleClose});
  };

  const label = field?.label ?? '';

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      {field && (
        <>
          <DialogTitle>
            {t('agents:edit.credentials.sections.reset.dialog.title', 'Reset {{label}}?', {label})}
          </DialogTitle>
          <DialogContent>
            <DialogContentText sx={{mb: 2}}>
              {t(
                'agents:edit.credentials.sections.reset.dialog.message',
                'A new {{label}} will be set for this agent. The current {{label}} will be invalidated immediately.',
                {label: label.toLowerCase()},
              )}
            </DialogContentText>
            <Alert severity="warning" sx={{mb: 2}}>
              {t(
                'agents:edit.credentials.sections.reset.dialog.disclaimer',
                'This action cannot be undone. The tokens and sessions this agent holds will be ended as soon as you confirm.',
              )}
            </Alert>
            <Stack spacing={2}>
              <FormControl fullWidth>
                <FormLabel sx={{mb: 0.5}}>
                  {t('agents:edit.credentials.sections.reset.dialog.newValue', 'New {{label}}', {label})}
                </FormLabel>
                <CredentialFieldInput
                  id={`agent-credential-new-${field.fieldName}`}
                  name={`new-${field.fieldName}`}
                  value={newValue}
                  placeholder={t(
                    'agents:edit.credentials.sections.reset.dialog.newValuePlaceholder',
                    'Enter new {{label}}',
                    {
                      label: label.toLowerCase(),
                    },
                  )}
                  required
                  error={false}
                  color="primary"
                  onChange={(e) => {
                    setNewValue(e.target.value);
                    setMismatch(false);
                    updateCredentials.reset();
                  }}
                  inputRef={null}
                />
              </FormControl>
              <FormControl fullWidth>
                <FormLabel sx={{mb: 0.5}}>
                  {t('agents:edit.credentials.sections.reset.dialog.confirmValue', 'Confirm {{label}}', {label})}
                </FormLabel>
                <CredentialFieldInput
                  id={`agent-credential-confirm-${field.fieldName}`}
                  name={`confirm-${field.fieldName}`}
                  value={confirmValue}
                  placeholder={t(
                    'agents:edit.credentials.sections.reset.dialog.confirmValuePlaceholder',
                    'Confirm new {{label}}',
                    {label: label.toLowerCase()},
                  )}
                  required
                  error={mismatch}
                  helperText={
                    mismatch
                      ? t('agents:edit.credentials.sections.reset.dialog.mismatch', 'Values do not match.')
                      : undefined
                  }
                  color={mismatch ? 'error' : 'primary'}
                  onChange={(e) => {
                    setConfirmValue(e.target.value);
                    setMismatch(false);
                    updateCredentials.reset();
                  }}
                  inputRef={null}
                />
              </FormControl>
            </Stack>
            {updateCredentials.error && (
              <Alert severity="error" sx={{mt: 2}}>
                {getErrorMessage(
                  updateCredentials.error,
                  tForErrors,
                  'updateCredentials.error',
                  'Failed to update credentials. Please try again.',
                )}
              </Alert>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={handleClose} disabled={updateCredentials.isPending}>
              {t('common:actions.cancel')}
            </Button>
            <Button
              variant="contained"
              color="error"
              onClick={handleSave}
              disabled={updateCredentials.isPending || newValue.trim() === ''}
            >
              {updateCredentials.isPending
                ? t('agents:edit.credentials.sections.reset.dialog.resetting', 'Resetting…')
                : t('agents:edit.credentials.sections.reset.dialog.resetButton', 'Reset {{label}}', {label})}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}
