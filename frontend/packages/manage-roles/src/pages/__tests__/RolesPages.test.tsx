// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter, Route, Routes, useLocation} from 'react-router';
import {describe, expect, it, vi} from 'vitest';
import {RolesListPage, RoleCreateDialog, RoleDetailsPage} from '../../index';

vi.mock('@thunderid/react', () => ({
  RoleList: ({
    actions,
    onSelect,
    title,
  }: {
    actions: ReactNode;
    onSelect: (item: {id: string}) => void;
    title: string;
  }): JSX.Element => (
    <div>
      <h1>{title}</h1>
      {actions}
      <button type="button" onClick={() => onSelect({id: 'x-1'})}>
        pick
      </button>
    </div>
  ),
  RoleCreate: ({onCancel, onCreated}: {onCancel: () => void; onCreated: (item: {id: string}) => void}): JSX.Element => (
    <div>
      <button type="button" onClick={() => onCreated({id: 'x-9'})}>
        create
      </button>
      <button type="button" onClick={onCancel}>
        cancel
      </button>
    </div>
  ),
  RoleAssignments: ({
    administrationMode,
    editable,
    roleId,
  }: {
    administrationMode: string;
    editable: boolean;
    roleId: string;
  }): JSX.Element => (
    <p>
      assignments of {roleId} {editable ? 'editable' : 'read only'} through {administrationMode}
    </p>
  ),
  RolePermissions: ({roleId}: {roleId: string}): JSX.Element => <p>permissions of {roleId}</p>,
}));

vi.mock('../../api/useGetRole', () => ({
  default: (id: string) => ({data: {description: 'Can sign in', id, name: 'Viewer'}}),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string, fallback?: string) => (typeof fallback === 'string' ? fallback : key)}),
}));

function Location(): JSX.Element {
  return <p data-testid="location">{useLocation().pathname}</p>;
}

const renderAt = (path: string, element: JSX.Element): void => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/roles" element={element} />
        <Route path="/roles/new" element={element} />
        <Route path="/roles/:id" element={element} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
};

describe('RolesListPage', () => {
  it('opens the role an admin picks', () => {
    renderAt('/roles', <RolesListPage />);
    fireEvent.click(screen.getByRole('button', {name: 'pick'}));
    expect(screen.getByTestId('location').textContent).toBe('/roles/x-1');
  });

  it('links to the create dialog only when admins may add', () => {
    renderAt('/roles', <RolesListPage canAdd />);
    expect(screen.getByRole('link', {name: 'Add'}).getAttribute('href')).toBe('/roles/new');
  });

  it('shows no add button by default', () => {
    renderAt('/roles', <RolesListPage />);
    expect(screen.queryByRole('link')).toBeNull();
  });
});

describe('RoleCreateDialog', () => {
  it('opens the new role once it is created', () => {
    renderAt('/roles/new', <RoleCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: 'create'}));
    expect(screen.getByTestId('location').textContent).toBe('/roles/x-9');
  });

  it('returns to the list when cancelled', () => {
    renderAt('/roles/new', <RoleCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: 'cancel'}));
    expect(screen.getByTestId('location').textContent).toBe('/roles');
  });
});

describe('RoleDetailsPage', () => {
  it('shows the role, its permissions and its assignments', () => {
    renderAt('/roles/r-1', <RoleDetailsPage editable />);
    expect(screen.getByRole('heading', {name: 'Viewer'})).toBeTruthy();
    expect(screen.getByText('permissions of r-1')).toBeTruthy();
    expect(screen.getByText('assignments of r-1 editable through flow')).toBeTruthy();
    expect(screen.getByRole('link', {name: 'Back'}).getAttribute('href')).toBe('/roles');
  });
});
