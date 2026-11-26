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
  FormControlLabel,
  FormHelperText,
  FormLabel,
  Radio,
  RadioGroup,
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

/**
 * Emailing the owner a link to set the new value, offered in the dialog beside setting it directly.
 */
export interface CredentialResetLinkOption {
  /** Sends the link. When it rejects, the dialog shows the error and stays open. */
  onSend: () => Promise<unknown>;
  /** Turns the error `onSend` rejected with into the message the dialog shows. */
  getErrorMessage: (error: Error) => string;
  /** Why a link cannot be sent now. The option is shown disabled with this reason. */
  heldReason?: string;
}

type ResetMethod = 'set' | 'link';

export interface CredentialResetDialogProps {
  open: boolean;
  field: CredentialResetField | null;
  /** Sets the new value. When it rejects, the dialog shows the error and stays open. */
  onReset: (field: CredentialResetField, newValue: string) => Promise<unknown>;
  /** Turns the error `onReset` rejected with into the message the dialog shows. */
  getErrorMessage: (error: Error) => string;
  /** Offers to email a reset link instead of setting the value here. Not given, the dialog only sets it. */
  resetLink?: CredentialResetLinkOption;
  onClose: () => void;
}

/**
 * Asks for a new value twice and hands it to `onReset`, or, when `resetLink` is given and chosen, sends a link
 * for the owner to set it. The caller owns the update calls, so one dialog serves a user and an agent alike.
 */
export default function CredentialResetDialog({
  open,
  field,
  onReset,
  getErrorMessage,
  resetLink = undefined,
  onClose,
}: CredentialResetDialogProps): JSX.Element {
  const {t} = useTranslation('common');

  const [method, setMethod] = useState<ResetMethod>('set');
  const [newValue, setNewValue] = useState('');
  const [confirmValue, setConfirmValue] = useState('');
  const [mismatch, setMismatch] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const sendsLink = method === 'link' && resetLink !== undefined;

  const handleClose = (): void => {
    if (isSubmitting) return;
    setMethod('set');
    setNewValue('');
    setConfirmValue('');
    setMismatch(false);
    setError(null);
    onClose();
  };

  const submit = async (action: () => Promise<unknown>): Promise<void> => {
    setIsSubmitting(true);
    try {
      await action();
      setIsSubmitting(false);
      handleClose();
    } catch (e) {
      setError(e as Error);
      setIsSubmitting(false);
    }
  };

  const handleSave = async (): Promise<void> => {
    if (!field) return;

    if (sendsLink) {
      await submit(resetLink.onSend);
      return;
    }

    if (newValue.trim() === '') return;

    if (newValue !== confirmValue) {
      setMismatch(true);
      return;
    }

    await submit(() => onReset(field, newValue));
  };

  // A failed save is stale once the form changes.
  const handleChange = (set: (value: string) => void) => (e: React.ChangeEvent<HTMLInputElement>) => {
    set(e.target.value);
    setMismatch(false);
    setError(null);
  };

  const label = field?.label ?? '';
  const lowerLabel = label.toLowerCase();
  const linkHeld = Boolean(resetLink?.heldReason);

  let submitLabel = t('credentialReset.dialog.resetButton', 'Reset {{label}}', {label});
  if (isSubmitting) {
    submitLabel = sendsLink
      ? t('credentialReset.dialog.sending', 'Sending…')
      : t('credentialReset.dialog.resetting', 'Resetting…');
  } else if (sendsLink) {
    submitLabel = t('credentialReset.dialog.link.sendButton', 'Send reset link');
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      {field && (
        <>
          <DialogTitle>{t('credentialReset.dialog.title', 'Reset {{label}}?', {label})}</DialogTitle>
          <DialogContent>
            {resetLink && (
              <FormControl sx={{mb: 2}}>
                <FormLabel id={`credential-reset-method-${field.fieldName}`} sx={{mb: 0.5}}>
                  {t('credentialReset.dialog.method.label', 'How should the {{label}} be reset?', {
                    label: lowerLabel,
                  })}
                </FormLabel>
                <RadioGroup
                  aria-labelledby={`credential-reset-method-${field.fieldName}`}
                  value={method}
                  onChange={(e) => {
                    setMethod(e.target.value as ResetMethod);
                    setError(null);
                  }}
                >
                  <FormControlLabel
                    value="set"
                    control={<Radio />}
                    label={t('credentialReset.dialog.method.set', 'Set a new {{label}} now', {label: lowerLabel})}
                    disabled={isSubmitting}
                  />
                  <FormControlLabel
                    value="link"
                    control={<Radio />}
                    label={t('credentialReset.dialog.method.link', 'Email the user a link to set a new {{label}}', {
                      label: lowerLabel,
                    })}
                    disabled={isSubmitting || linkHeld}
                  />
                </RadioGroup>
                {/* Text, not a tooltip: a disabled option cannot be focused, so a keyboard user would never see one. */}
                {linkHeld && <FormHelperText sx={{mx: 0}}>{resetLink.heldReason}</FormHelperText>}
              </FormControl>
            )}
            {sendsLink ? (
              <>
                <DialogContentText sx={{mb: 2}}>
                  {t(
                    'credentialReset.dialog.link.message',
                    'The user will receive an email with a link to set a new {{label}}.',
                    {label: lowerLabel},
                  )}
                </DialogContentText>
                <Alert severity="warning">
                  {t(
                    'credentialReset.dialog.link.warning',
                    '{{label}} stops working once the email is sent, and the user is signed out everywhere. It stays unusable until the user sets a new value from the link.',
                    {label},
                  )}
                </Alert>
              </>
            ) : (
              <>
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
                    <FormLabel sx={{mb: 0.5}}>
                      {t('credentialReset.dialog.newValue', 'New {{label}}', {label})}
                    </FormLabel>
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
              </>
            )}
            {error && (
              <Alert severity="error" sx={{mt: 2}} data-testid="credential-reset-error">
                {sendsLink ? resetLink.getErrorMessage(error) : getErrorMessage(error)}
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
              disabled={isSubmitting || (!sendsLink && newValue.trim() === '')}
            >
              {submitLabel}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}
