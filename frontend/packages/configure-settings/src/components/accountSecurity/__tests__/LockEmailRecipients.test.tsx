// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {describe, it, expect} from 'vitest';
import {toAccountLockForm, type AccountLockForm} from '../../../utils/accountLockPayload';
import LockEmailRecipients from '../LockEmailRecipients';

function Editor({initial = {}}: {initial?: Partial<AccountLockForm>}) {
  const [form, setForm] = useState({...toAccountLockForm(undefined, 'user'), ...initial});
  return <LockEmailRecipients form={form} errors={{}} onChange={(patch) => setForm({...form, ...patch})} />;
}

describe('lock email recipients', () => {
  it('defaults to the email attribute', () => {
    render(<Editor />);
    expect(screen.getByRole('textbox', {name: 'Recipient attribute'})).toHaveValue('email');
  });

  it('accepts any attribute name, not only a listed one', async () => {
    render(<Editor />);
    const input = screen.getByRole('textbox', {name: 'Recipient attribute'});
    await userEvent.clear(input);
    await userEvent.type(input, 'workEmail');
    expect(input).toHaveValue('workEmail');
  });

  it('shows a blocking message in place of the hint', () => {
    const form = toAccountLockForm(undefined, 'user');
    render(
      <LockEmailRecipients
        form={{...form, notifyRecipientAttribute: ''}}
        errors={{notifyRecipientAttribute: 'Enter a recipient attribute.'}}
        onChange={() => undefined}
      />,
    );
    expect(screen.getByText('Enter a recipient attribute.')).toBeVisible();
    expect(screen.queryByText('The user attribute the lock email is sent to.')).not.toBeInTheDocument();
  });
});
