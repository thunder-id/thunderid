// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter, Route, Routes, useLocation} from 'react-router';
import {describe, expect, it, vi} from 'vitest';
import {UsersListPage, UserCreateDialog, UserDetailsPage} from '../../index';

vi.mock('@thunderid/react', () => ({
  UserList: ({
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
  UserCreate: ({
    mode,
    onCancel,
    onCreated,
  }: {
    mode: string;
    onCancel: () => void;
    onCreated: (item: {id: string}) => void;
  }): JSX.Element => (
    <div>
      <button type="button" onClick={() => onCreated({id: 'x-9'})}>
        create {mode}
      </button>
      <button type="button" onClick={onCancel}>
        cancel
      </button>
    </div>
  ),
  UserDetails: ({
    administrationMode,
    deletable,
    editable,
    lifecycleActions,
    onDeleted,
    userId,
  }: {
    administrationMode: string;
    deletable: boolean;
    editable: boolean;
    lifecycleActions: boolean;
    onDeleted: () => void;
    userId: string;
  }): JSX.Element => (
    <>
      <button type="button" onClick={onDeleted}>
        user {userId} {editable ? 'editable' : 'read only'} {deletable ? 'deletable' : 'kept'}
      </button>
      <p>
        through {administrationMode}, lifecycle {lifecycleActions ? 'on' : 'off'}
      </p>
    </>
  ),
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
        <Route path="/users" element={element} />
        <Route path="/users/new" element={element} />
        <Route path="/users/:id" element={element} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
};

describe('UsersListPage', () => {
  it('opens the user an admin picks', () => {
    renderAt('/users', <UsersListPage />);
    fireEvent.click(screen.getByRole('button', {name: 'pick'}));
    expect(screen.getByTestId('location').textContent).toBe('/users/x-1');
  });

  it('links to the create dialog when admins may add people', () => {
    renderAt('/users', <UsersListPage addAction="invite" />);
    expect(screen.getByRole('link', {name: 'Add'}).getAttribute('href')).toBe('/users/new');
  });

  it('shows no add button without an add action', () => {
    renderAt('/users', <UsersListPage title="People" />);
    expect(screen.getByRole('heading', {name: 'People'})).toBeTruthy();
    expect(screen.queryByRole('link')).toBeNull();
  });
});

describe('UserCreateDialog', () => {
  it('opens the new user once it is created', () => {
    renderAt('/users/new', <UserCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: /create/}));
    expect(screen.getByTestId('location').textContent).toBe('/users/x-9');
  });

  it('returns to the list when cancelled', () => {
    renderAt('/users/new', <UserCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: 'cancel'}));
    expect(screen.getByTestId('location').textContent).toBe('/users');
  });
});

it('creates users by invitation unless told to create them directly', () => {
  renderAt('/users/new', <UserCreateDialog />);
  expect(screen.getByRole('button', {name: 'create invite'})).toBeTruthy();
});

it('returns to the list when the invite flow is closed', () => {
  renderAt('/users/new', <UserCreateDialog />);
  fireEvent.click(screen.getByRole('button', {name: 'Close'}));
  expect(screen.getByTestId('location').textContent).toBe('/users');
});

describe('UserDetailsPage', () => {
  it('shows the user from the route, with the actions it is given', () => {
    renderAt('/users/u-1', <UserDetailsPage editable />);
    expect(screen.getByRole('button', {name: 'user u-1 editable kept'})).toBeTruthy();
    expect(screen.getByRole('link', {name: 'Back'}).getAttribute('href')).toBe('/users');
  });

  it('returns to the list once the user is deleted', () => {
    renderAt('/users/u-1', <UserDetailsPage deletable />);
    fireEvent.click(screen.getByRole('button', {name: 'user u-1 read only deletable'}));
    expect(screen.getByTestId('location').textContent).toBe('/users');
  });

  it('deletes through administration flows and offers no lifecycle actions by default', () => {
    renderAt('/users/u-1', <UserDetailsPage />);
    expect(screen.getByText('through flow, lifecycle off')).toBeTruthy();
  });

  it('passes the administration mode and lifecycle actions it is given', () => {
    renderAt('/users/u-1', <UserDetailsPage administrationMode="native" lifecycleActions />);
    expect(screen.getByText('through native, lifecycle on')).toBeTruthy();
  });
});
