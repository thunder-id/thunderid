// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter, Route, Routes, useLocation} from 'react-router';
import {describe, expect, it, vi} from 'vitest';
import {GroupsListPage, GroupCreateDialog, GroupDetailsPage} from '../../index';

vi.mock('@thunderid/react', () => ({
  GroupList: ({
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
  GroupCreate: ({
    onCancel,
    onCreated,
  }: {
    onCancel: () => void;
    onCreated: (item: {id: string}) => void;
  }): JSX.Element => (
    <div>
      <button type="button" onClick={() => onCreated({id: 'x-9'})}>
        create
      </button>
      <button type="button" onClick={onCancel}>
        cancel
      </button>
    </div>
  ),
  GroupMembers: ({
    administrationMode,
    editable,
    groupId,
  }: {
    administrationMode: string;
    editable: boolean;
    groupId: string;
  }): JSX.Element => (
    <p>
      members of {groupId} {editable ? 'editable' : 'read only'} through {administrationMode}
    </p>
  ),
}));

vi.mock('../../api/useGetGroup', () => ({
  default: (id: string) => ({data: {description: 'The admins', id, name: 'Admins'}}),
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
        <Route path="/groups" element={element} />
        <Route path="/groups/new" element={element} />
        <Route path="/groups/:id" element={element} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
};

describe('GroupsListPage', () => {
  it('opens the group an admin picks', () => {
    renderAt('/groups', <GroupsListPage />);
    fireEvent.click(screen.getByRole('button', {name: 'pick'}));
    expect(screen.getByTestId('location').textContent).toBe('/groups/x-1');
  });

  it('links to the create dialog only when admins may add', () => {
    renderAt('/groups', <GroupsListPage canAdd />);
    expect(screen.getByRole('link', {name: 'Add'}).getAttribute('href')).toBe('/groups/new');
  });

  it('shows no add button by default', () => {
    renderAt('/groups', <GroupsListPage />);
    expect(screen.queryByRole('link')).toBeNull();
  });
});

describe('GroupCreateDialog', () => {
  it('opens the new group once it is created', () => {
    renderAt('/groups/new', <GroupCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: 'create'}));
    expect(screen.getByTestId('location').textContent).toBe('/groups/x-9');
  });

  it('returns to the list when cancelled', () => {
    renderAt('/groups/new', <GroupCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: 'cancel'}));
    expect(screen.getByTestId('location').textContent).toBe('/groups');
  });
});

describe('GroupDetailsPage', () => {
  it('shows the group and its members, read only unless told otherwise', () => {
    renderAt('/groups/g-1', <GroupDetailsPage />);
    expect(screen.getByRole('heading', {name: 'Admins'})).toBeTruthy();
    expect(screen.getByText('The admins')).toBeTruthy();
    expect(screen.getByText('members of g-1 read only through flow')).toBeTruthy();
    expect(screen.getByRole('link', {name: 'Back'}).getAttribute('href')).toBe('/groups');
  });

  it('lets admins change the members when editable', () => {
    renderAt('/groups/g-1', <GroupDetailsPage editable />);
    expect(screen.getByText('members of g-1 editable through flow')).toBeTruthy();
  });

  it('removes members through the API directly when told to', () => {
    renderAt('/groups/g-1', <GroupDetailsPage editable administrationMode="native" />);
    expect(screen.getByText('members of g-1 editable through native')).toBeTruthy();
  });
});
