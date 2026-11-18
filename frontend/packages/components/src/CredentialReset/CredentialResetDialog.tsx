// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CredentialFieldInput from '../CredentialFieldInput/CredentialFieldInput';

/**
 * A credential the schema declares, such as a password, and the name to show for it.
 */
export interface CredentialResetField {
  fieldName: string;
  label: string;
}

export interface CredentialResetDialogProps {
  open: boolean;
  field: CredentialResetField | null;
  /** Sets the new value. When it rejects, the dialog shows the error and stays open. */
  onReset: (field: CredentialResetField, newValue: string) => Promise<unknown>;
  /** Turns the error `onReset` rejected with into the message the dialog shows. */
  getErrorMessage: (error: Error) => string;
  onClose: () => void;
}

/**
 * Asks for a new value twice and hands it to `onReset`. The caller owns the update call, so one dialog serves a
 * user and an agent alike.
 */
export default function CredentialResetDialog({
  open,
  field,
  onReset,
  getErrorMessage,
  onClose,
}: CredentialResetDialogProps): JSX.Element {
  const {t} = useTranslation('common');

  const [newValue, setNewValue] = useState('');
  const [confirmValue, setConfirmValue] = useState('');
  const [mismatch, setMismatch] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const handleClose = (): void => {
    setNewValue('');
    setConfirmValue('');
    setMismatch(false);
    setError(null);
    onClose();
  };

  const handleSave = async (): Promise<void> => {
    if (!field || newValue.trim() === '') return;

    if (newValue !== confirmValue) {
      setMismatch(true);
      return;
    }

    setIsSubmitting(true);
    try {
      await onReset(field, newValue);
      handleClose();
    } catch (e) {
      setError(e as Error);
    } finally {
      setIsSubmitting(false);
    }
  };

  // A failed save is stale once the form changes.
  const handleChange = (set: (value: string) => void) => (e: React.ChangeEvent<HTMLInputElement>) => {
    set(e.target.value);
    setMismatch(false);
    setError(null);
  };

  const label = field?.label ?? '';
  const lowerLabel = label.toLowerCase();

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      {field && (
        <>
          <DialogTitle>{t('credentialReset.dialog.title', 'Reset {{label}}?', {label})}</DialogTitle>
          <DialogContent>
            <DialogContentText sx={{mb: 2}}>
              {t(
                'credentialReset.dialog.message',
                'A new {{label}} will be set. The current {{label}} will be invalidated immediately.',
                {label: lowerLabel},
              )}
            </DialogContentText>
            <Alert severity="warning" sx={{mb: 2}}>
              {t(
                'credentialReset.dialog.disclaimer',
                'This action cannot be undone. The tokens and sessions issued with the current {{label}} will be ended as soon as you confirm.',
                {label: lowerLabel},
              )}
            </Alert>
            <Stack spacing={2}>
              <FormControl fullWidth>
                <FormLabel sx={{mb: 0.5}}>{t('credentialReset.dialog.newValue', 'New {{label}}', {label})}</FormLabel>
                <CredentialFieldInput
                  id={`credential-new-${field.fieldName}`}
                  name={`new-${field.fieldName}`}
                  value={newValue}
                  placeholder={t('credentialReset.dialog.newValuePlaceholder', 'Enter new {{label}}', {
                    label: lowerLabel,
                  })}
                  required
                  error={false}
                  color="primary"
                  onChange={handleChange(setNewValue)}
                  inputRef={null}
                />
              </FormControl>
              <FormControl fullWidth>
                <FormLabel sx={{mb: 0.5}}>
                  {t('credentialReset.dialog.confirmValue', 'Confirm {{label}}', {label})}
                </FormLabel>
                <CredentialFieldInput
                  id={`credential-confirm-${field.fieldName}`}
                  name={`confirm-${field.fieldName}`}
                  value={confirmValue}
                  placeholder={t('credentialReset.dialog.confirmValuePlaceholder', 'Confirm new {{label}}', {
                    label: lowerLabel,
                  })}
                  required
                  error={mismatch}
                  helperText={mismatch ? t('credentialReset.dialog.mismatch', 'Values do not match.') : undefined}
                  color={mismatch ? 'error' : 'primary'}
                  onChange={handleChange(setConfirmValue)}
                  inputRef={null}
                />
              </FormControl>
            </Stack>
            {error && (
              <Alert severity="error" sx={{mt: 2}}>
                {getErrorMessage(error)}
              </Alert>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={handleClose} disabled={isSubmitting}>
              {t('actions.cancel', 'Cancel')}
            </Button>
            <Button
              variant="contained"
              color="error"
              onClick={() => void handleSave()}
              disabled={isSubmitting || newValue.trim() === ''}
            >
              {isSubmitting
                ? t('credentialReset.dialog.resetting', 'Resetting…')
                : t('credentialReset.dialog.resetButton', 'Reset {{label}}', {label})}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}
