// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, expect, it, vi} from 'vitest';
import AdministratorsListPage from '../AdministratorsListPage';

vi.mock('@thunderid/configure-users', () => ({
  UsersListPage: ({title = undefined, notice = undefined}: {title?: string; notice?: ReactNode}) => (
    <div data-testid="users-list-page">
      <h1>{title}</h1>
      {notice}
    </div>
  ),
}));

describe('AdministratorsListPage', () => {
  it('lists the users as the administrators of this console, not the users of the gateways', () => {
    render(<AdministratorsListPage />);

    expect(screen.getByRole('heading', {name: 'Administrators'})).toBeInTheDocument();
    expect(screen.getByTestId('administrators-notice')).toHaveTextContent(/not users of your gateways/);
  });
});
