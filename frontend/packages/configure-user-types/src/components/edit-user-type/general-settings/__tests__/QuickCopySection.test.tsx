// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {ApiUserType} from '../../../../types/user-types';
import QuickCopySection from '../QuickCopySection';

const userType: ApiUserType = {
  id: 'schema-123',
  handle: 'employee',
  displayName: 'Employee',
  ouId: 'root-ou',
  allowSelfRegistration: false,
  schema: {},
};

describe('QuickCopySection', () => {
  it('shows the handle and the id as read-only fields', () => {
    render(<QuickCopySection userType={userType} copiedField={null} onCopyToClipboard={vi.fn()} />);

    expect(screen.getByDisplayValue('employee')).toHaveAttribute('readonly');
    expect(screen.getByDisplayValue('schema-123')).toHaveAttribute('readonly');
  });

  it('copies the handle', async () => {
    const user = userEvent.setup();
    const onCopyToClipboard = vi.fn().mockResolvedValue(undefined);
    render(<QuickCopySection userType={userType} copiedField={null} onCopyToClipboard={onCopyToClipboard} />);

    await user.click(screen.getByRole('button', {name: /copy user type handle/i}));

    expect(onCopyToClipboard).toHaveBeenCalledWith('employee', 'user_type_handle');
  });

  it('copies the id', async () => {
    const user = userEvent.setup();
    const onCopyToClipboard = vi.fn().mockResolvedValue(undefined);
    render(<QuickCopySection userType={userType} copiedField={null} onCopyToClipboard={onCopyToClipboard} />);

    await user.click(screen.getByRole('button', {name: /copy user type id/i}));

    expect(onCopyToClipboard).toHaveBeenCalledWith('schema-123', 'user_type_id');
  });
});
