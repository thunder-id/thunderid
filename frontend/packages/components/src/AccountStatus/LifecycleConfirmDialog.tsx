// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
} from '@wso2/oxygen-ui';
import type {TFunction} from 'i18next';
import {useState, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {lifecycleActionLabel, type LifecycleAction, type LifecycleCategory} from './lifecycleAction';

export interface LifecycleConfirmDialogProps {
  open: boolean;
  operation: LifecycleAction;
  category: LifecycleCategory;
  submitting: boolean;
  error?: string;
  onCancel: () => void;
  /**
   * Confirms the operation, with the operator note when one was given on a suspend.
   */
  onConfirm: (operatorNote?: string) => void;
}

interface DialogCopy {
  title: string;
  guidance?: string;
  body: string;
}

function suspendCopy(category: LifecycleCategory, t: TFunction): DialogCopy {
  switch (category) {
    case 'agent':
      return {
        body: t(
          'common:accountStatus.suspend.confirmBody.agent',
          'The agent will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked. You can still update the agent details and credentials while it is suspended.',
        ),
        title: t('common:accountStatus.suspend.confirmTitle.agent', 'Suspend agent'),
      };
    case 'application':
      return {
        body: t(
          'common:accountStatus.suspend.confirmBody.application',
          'The application will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked.',
        ),
        title: t('common:accountStatus.suspend.confirmTitle.application', 'Suspend application'),
      };
    default:
      return {
        body: t(
          'common:accountStatus.suspend.confirmBody.user',
          'The account will no longer be able to sign in. Existing sessions and tokens will be revoked.',
        ),
        title: t('common:accountStatus.suspend.confirmTitle.user', 'Suspend user'),
      };
  }
}

function unsuspendCopy(category: LifecycleCategory, t: TFunction): DialogCopy {
  switch (category) {
    case 'agent':
      return {
        body: t(
          'common:accountStatus.unsuspend.confirmBody.agent',
          "This action will remove the suspension and restore the agent's access, unless the agent is also locked.",
        ),
        guidance: t(
          'common:accountStatus.unsuspend.confirmGuidance.agent',
          'Before unsuspending, rotate any potentially compromised agent secrets to prevent their reuse.',
        ),
        title: t('common:accountStatus.unsuspend.confirmTitle.agent', 'Unsuspend agent'),
      };
    case 'application':
      return {
        body: t(
          'common:accountStatus.unsuspend.confirmBody.application',
          "This action will remove the suspension and restore the application's access.",
        ),
        guidance: t(
          'common:accountStatus.unsuspend.confirmGuidance.application',
          'Before unsuspending, rotate any potentially compromised application secrets to prevent their reuse.',
        ),
        title: t('common:accountStatus.unsuspend.confirmTitle.application', 'Unsuspend application'),
      };
    default:
      return {
        body: t(
          'common:accountStatus.unsuspend.confirmBody.user',
          "The user's account will remain locked after unsuspension. To regain access, the user must complete account recovery or an administrator must unlock the account.",
        ),
        guidance: t(
          'common:accountStatus.unsuspend.confirmGuidance.user',
          "Before unsuspending, verify the user's contact information and reset any potentially compromised credentials.",
        ),
        title: t('common:accountStatus.unsuspend.confirmTitle.user', 'Unsuspend user'),
      };
  }
}

/** Only users are offered unsuspend and unlock. */
function unsuspendAndUnlockCopy(t: TFunction): DialogCopy {
  return {
    body: t(
      'common:accountStatus.unsuspendAndUnlock.confirmBody',
      'This action will remove the suspension and unlock the account, allowing the user to sign in again.',
    ),
    guidance: t(
      'common:accountStatus.unsuspendAndUnlock.confirmGuidance',
      "Before restoring access, verify the user's contact information and reset any potentially compromised credentials.",
    ),
    title: t('common:accountStatus.unsuspendAndUnlock.confirmTitle', 'Unsuspend and unlock user'),
  };
}

/** Matches the backend's operator note limit. */
const OPERATOR_NOTE_MAX_LENGTH = 100;

/**
 * Confirmation for suspend, unsuspend and unsuspend-and-unlock. Unlock runs without one.
 */
export default function LifecycleConfirmDialog({
  open,
  operation,
  category,
  submitting,
  error = undefined,
  onCancel,
  onConfirm,
}: LifecycleConfirmDialogProps): JSX.Element {
  const {t} = useTranslation('common');
  const [operatorNote, setOperatorNote] = useState('');

  const collectsOperatorNote = operation === 'suspend' && category !== 'application';

  const copy = ((): DialogCopy => {
    switch (operation) {
      case 'unsuspend':
        return unsuspendCopy(category, t);
      case 'unsuspendAndUnlock':
        return unsuspendAndUnlockCopy(t);
      default:
        return suspendCopy(category, t);
    }
  })();

  const confirmLabel = lifecycleActionLabel(operation, t);

  return (
    <Dialog
      open={open}
      onClose={submitting ? undefined : onCancel}
      maxWidth="xs"
      fullWidth
      // Clear the note once the exit fade ends, so the next open starts empty.
      slotProps={{transition: {onExited: (): void => setOperatorNote('')}}}
    >
      <DialogTitle>{copy.title}</DialogTitle>
      <DialogContent>
        {copy.guidance && <DialogContentText sx={{mb: 2}}>{copy.guidance}</DialogContentText>}
        <DialogContentText>{copy.body}</DialogContentText>
        {collectsOperatorNote && (
          <TextField
            fullWidth
            size="small"
            sx={{mt: 2}}
            label={t('common:accountStatus.suspend.operatorNoteLabel', 'Operator note (optional)')}
            helperText={t(
              'common:accountStatus.suspend.operatorNoteHelp',
              'Shown to anyone who can read this record. It is never shown on a sign-in attempt.',
            )}
            value={operatorNote}
            disabled={submitting}
            slotProps={{htmlInput: {maxLength: OPERATOR_NOTE_MAX_LENGTH}}}
            onChange={(event: ChangeEvent<HTMLInputElement>): void => setOperatorNote(event.target.value)}
          />
        )}
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={submitting}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          onClick={(): void => onConfirm(collectsOperatorNote ? operatorNote.trim() || undefined : undefined)}
          variant="contained"
          color={operation === 'suspend' ? 'error' : 'primary'}
          disabled={submitting}
          startIcon={
            submitting ? (
              <CircularProgress size={16} aria-label={t('common:status.loading', 'Loading...')} />
            ) : undefined
          }
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
