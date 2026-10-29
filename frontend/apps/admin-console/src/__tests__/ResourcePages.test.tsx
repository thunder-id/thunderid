// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {JSX} from 'react';
import {MemoryRouter, Route, Routes, useLocation} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {ResourceCreatePage, ResourceDetailsPage, ResourceListPage} from '../pages/ResourcePages';

const granted: {isLoading: boolean; permissions: string[]} = {isLoading: false, permissions: []};

vi.mock('@thunderid/react', () => ({
  useGrantedPermissions: () => ({
    can: (required: string): boolean =>
      granted.permissions.some(
        (permission: string) => required === permission || required.startsWith(`${permission}:`),
      ),
    isLoading: granted.isLoading,
    permissions: granted.permissions,
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: string | {defaultValue?: string}): string =>
      typeof options === 'string' ? options : (options?.defaultValue ?? key),
  }),
}));

const {flag} = vi.hoisted(() => ({flag: (value: unknown): string => (value === true ? 'on' : 'off')}));

vi.mock('@thunderid/manage-users', () => ({
  UserCreateDialog: ({mode}: {mode: string}): JSX.Element => <p>user create {mode}</p>,
  UserDetailsPage: ({
    deletable,
    editable,
    lifecycleActions,
  }: {
    deletable: boolean;
    editable: boolean;
    lifecycleActions: boolean;
  }): JSX.Element => (
    <p>
      user details edit {flag(editable)} delete {flag(deletable)} lifecycle {flag(lifecycleActions)}
    </p>
  ),
  UsersListPage: ({addAction, title}: {addAction: string; title: string}): JSX.Element => (
    <p>
      user list {title} add {addAction}
    </p>
  ),
}));

vi.mock('@thunderid/manage-groups', () => ({
  GroupCreateDialog: (): JSX.Element => <p>group create</p>,
  GroupDetailsPage: ({editable}: {editable: boolean}): JSX.Element => <p>group details members {flag(editable)}</p>,
  GroupsListPage: ({canAdd}: {canAdd: boolean}): JSX.Element => <p>group list add {flag(canAdd)}</p>,
}));

vi.mock('@thunderid/manage-roles', () => ({
  RoleCreateDialog: (): JSX.Element => <p>role create</p>,
  RoleDetailsPage: ({editable}: {editable: boolean}): JSX.Element => <p>role details assign {flag(editable)}</p>,
  RolesListPage: ({canAdd}: {canAdd: boolean}): JSX.Element => <p>role list add {flag(canAdd)}</p>,
}));

vi.mock('@thunderid/manage-agents', () => ({
  AgentCreateDialog: (): JSX.Element => <p>agent create</p>,
  AgentDetailsPage: ({deletable, lifecycleActions}: {deletable: boolean; lifecycleActions: boolean}): JSX.Element => (
    <p>
      agent details delete {flag(deletable)} lifecycle {flag(lifecycleActions)}
    </p>
  ),
  AgentsListPage: ({canAdd}: {canAdd: boolean}): JSX.Element => <p>agent list add {flag(canAdd)}</p>,
}));

function Location(): JSX.Element {
  return <p data-testid="location">{useLocation().pathname}</p>;
}

const renderAt = (path: string, element: JSX.Element): void => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="*" element={element} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
};

const ALL_VIEWS: string[] = ['system:user:view', 'system:group:view', 'system:role:view', 'system:agent:view'];

describe('ResourcePages', () => {
  beforeEach(() => {
    granted.isLoading = false;
    granted.permissions = [...ALL_VIEWS];
  });

  it('shows a page the admin may view, with adding on', () => {
    renderAt('/groups', <ResourceListPage resource="groups" />);
    expect(screen.getByText('group list add on')).toBeInTheDocument();
  });

  it('lets admins create users directly', () => {
    renderAt('/users', <ResourceListPage resource="users" />);
    expect(screen.getByText('user list users add create')).toBeInTheDocument();
  });

  it('waits for the permissions before deciding', () => {
    granted.isLoading = true;
    renderAt('/users', <ResourceListPage resource="users" />);
    expect(screen.getByRole('progressbar', {name: 'Loading'})).toBeInTheDocument();
  });

  it('sends the admin to the first page they may view', () => {
    granted.permissions = ['system:role:view', 'system:group:view'];
    renderAt('/users', <ResourceListPage resource="users" />);
    expect(screen.getByTestId('location').textContent).toBe('/groups');
  });

  it('explains when the admin may view nothing', () => {
    granted.permissions = [];
    renderAt('/users', <ResourceListPage resource="users" />);
    expect(screen.getByRole('heading', {name: 'Nothing to manage'})).toBeInTheDocument();
  });

  it('opens the create dialog over the list', () => {
    renderAt('/roles/new', <ResourceCreatePage resource="roles" />);
    expect(screen.getByText('role list add on')).toBeInTheDocument();
    expect(screen.getByText('role create')).toBeInTheDocument();
  });

  it('adds users through the user onboarding flow', () => {
    renderAt('/users/new', <ResourceCreatePage resource="users" />);
    expect(screen.getByText('user create invite')).toBeInTheDocument();
  });

  it('lets admins edit and delete users', () => {
    renderAt('/users/u-1', <ResourceDetailsPage resource="users" />);
    expect(screen.getByText('user details edit on delete on lifecycle off')).toBeInTheDocument();
  });

  it.each([
    ['groups', 'group details members on'],
    ['roles', 'role details assign on'],
    ['agents', 'agent details delete on lifecycle off'],
  ] as const)('opens %s details with their actions', (resource, text) => {
    renderAt(`/${resource}/x-1`, <ResourceDetailsPage resource={resource} />);
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it('offers lifecycle actions only to admins who may read the server configuration', () => {
    granted.permissions = ['system'];
    renderAt('/users/u-1', <ResourceDetailsPage resource="users" />);
    expect(screen.getByText('user details edit on delete on lifecycle on')).toBeInTheDocument();
  });
});
