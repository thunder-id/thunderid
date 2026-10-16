// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {AccountLockForm} from '../../utils/accountLockPayload';

interface LockEmailRecipientsProps {
  form: AccountLockForm;
  errors: Partial<Record<keyof AccountLockForm, string>>;
  onChange: (patch: Partial<AccountLockForm>) => void;
  /** Whether the field is read-only. */
  disabled?: boolean;
}

/** Edits the user attribute the lock email is sent to. Any user type's attribute may be named. */
export default function LockEmailRecipients({
  form,
  errors,
  onChange,
  disabled = false,
}: LockEmailRecipientsProps): JSX.Element {
  const {t} = useTranslation();
  return (
    <TextField
      size="small"
      fullWidth
      label={t('settings:accountLock.notify.attribute', 'Recipient attribute')}
      value={form.notifyRecipientAttribute}
      disabled={disabled}
      error={Boolean(errors.notifyRecipientAttribute)}
      helperText={
        errors.notifyRecipientAttribute ??
        t('settings:accountLock.notify.attributeHint', 'The user attribute the lock email is sent to.')
      }
      slotProps={{htmlInput: {'data-testid': 'notify-recipient-attribute'}}}
      onChange={(event) => onChange({notifyRecipientAttribute: event.target.value})}
      sx={{'& .MuiInputBase-root': {height: 40}}}
    />
  );
}
