// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
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
  it('lists the users as the administrators of this console, and says project users are listed in their project', () => {
    render(<AdministratorsListPage />);

    expect(screen.getByRole('heading', {name: 'Administrators'})).toBeInTheDocument();
    expect(screen.getByTestId('administrators-notice')).toHaveTextContent(
      /users of a project are listed in that project/,
    );
  });

  it("lists the project's users, not the administrators, inside a project", () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <AdministratorsListPage />
      </ProjectScopeProvider>,
    );

    expect(screen.getByTestId('users-list-page')).toBeInTheDocument();
    expect(screen.queryByRole('heading', {name: 'Administrators'})).not.toBeInTheDocument();
    expect(screen.queryByTestId('administrators-notice')).not.toBeInTheDocument();
  });
});
