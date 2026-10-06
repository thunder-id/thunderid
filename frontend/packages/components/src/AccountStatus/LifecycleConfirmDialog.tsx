// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {LifecycleCategory} from '@thunderid/types';
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
import {useState, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {LifecycleAction} from './lifecycleAction';

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

const SUBJECT_NOUN: Record<LifecycleCategory, {key: string; fallback: string}> = {
  agent: {fallback: 'agent', key: 'common:accountStatus.subject.agent'},
  application: {fallback: 'application', key: 'common:accountStatus.subject.application'},
  user: {fallback: 'account', key: 'common:accountStatus.subject.user'},
};

/**
 * The longest note the field accepts.
 */
const OPERATOR_NOTE_MAX_LENGTH = 500;

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
  const noun = t(SUBJECT_NOUN[category].key, SUBJECT_NOUN[category].fallback);
  const [operatorNote, setOperatorNote] = useState('');

  const collectsOperatorNote = operation === 'suspend' && category !== 'application';

  const title = ((): string => {
    switch (operation) {
      case 'unsuspend':
        return t('common:accountStatus.unsuspend.confirmTitle', 'Unsuspend this {{noun}}?', {noun});
      case 'unsuspendAndUnlock':
        return t('common:accountStatus.unsuspendAndUnlock.confirmTitle', 'Unsuspend and unlock this {{noun}}?', {
          noun,
        });
      default:
        return t('common:accountStatus.suspend.confirmTitle', 'Suspend this {{noun}}?', {noun});
    }
  })();

  const body = ((): string => {
    switch (operation) {
      case 'unsuspend':
        // A user stays locked after unsuspend, so the copy says so.
        if (category === 'user') {
          return t(
            'common:accountStatus.unsuspend.confirmBody.user',
            'The suspension will be removed. The account remains locked until an administrator unlocks it or the user completes self-recovery.',
          );
        }
        if (category === 'application') {
          return t(
            'common:accountStatus.unsuspend.confirmBody.application',
            'The suspension will be removed, and the application can authenticate and obtain tokens again. Sessions and tokens revoked during the suspension are not restored.',
          );
        }
        return t(
          'common:accountStatus.unsuspend.confirmBody',
          'The suspension will be removed, and the agent can authenticate again unless it is also locked. Sessions and tokens revoked during the suspension are not restored.',
        );
      case 'unsuspendAndUnlock':
        // Offered to users only. Skips the post-suspension lock, so the credential check moves here.
        return t(
          'common:accountStatus.unsuspendAndUnlock.confirmBody',
          'The suspension and the lock will be removed, and the account can sign in right away. Sessions and tokens revoked during the suspension are not restored.',
        );
      default:
        // Per category, since only a user signs in. The copy avoids "lock".
        if (category === 'application') {
          return t(
            'common:accountStatus.suspend.confirmBody.application',
            'The application will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked.',
          );
        }
        if (category === 'agent') {
          return t(
            'common:accountStatus.suspend.confirmBody.agent',
            'The agent will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked. You can still update the agent details and credentials while it is suspended.',
          );
        }
        return t(
          'common:accountStatus.suspend.confirmBody',
          'The account will no longer be able to sign in. Existing sessions and tokens will be revoked.',
        );
    }
  })();

  const confirmLabel = ((): string => {
    switch (operation) {
      case 'unsuspend':
        return t('common:accountStatus.unsuspend.action', 'Unsuspend');
      case 'unsuspendAndUnlock':
        return t('common:accountStatus.unsuspendAndUnlock.action', 'Unsuspend and unlock');
      default:
        return t('common:accountStatus.suspend.action', 'Suspend');
    }
  })();

  return (
    <Dialog
      open={open}
      onClose={submitting ? undefined : onCancel}
      maxWidth="xs"
      fullWidth
      // Clear the note once the exit fade ends, so the next open starts empty.
      slotProps={{transition: {onExited: (): void => setOperatorNote('')}}}
    >
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <DialogContentText>{body}</DialogContentText>
        {/* Only user and agent suspension collects an operator note, and it is optional there. The field says where the value
            goes, because an operator writing an incident note needs to know who can read it back:
            anyone who can read this record, and nobody else. It never reaches a sign-in response. */}
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
          {t('common:accountStatus.cancel', 'Cancel')}
        </Button>
        <Button
          onClick={(): void => onConfirm(collectsOperatorNote ? operatorNote.trim() || undefined : undefined)}
          variant="contained"
          color={operation === 'suspend' ? 'error' : 'primary'}
          disabled={submitting}
          startIcon={submitting ? <CircularProgress size={16} /> : undefined}
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
