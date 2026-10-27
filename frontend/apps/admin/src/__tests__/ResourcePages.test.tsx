// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {JSX} from 'react';
import {MemoryRouter, Route, Routes, useLocation} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {ResourceCreatePage, ResourceDetailsPage, ResourceListPage} from '../pages/ResourcePages';

const mockPages: Record<string, {actions: Record<string, boolean>; enabled: boolean}> = {};

vi.mock('../hooks/useAdminAppConfig', () => ({
  default: () => {
    const page = (resource: string) => mockPages[resource];

    return {
      isLoading: false,
      page,
      visibleResources: ['users', 'groups', 'roles', 'agents'].filter((resource) => page(resource)?.enabled),
    };
  },
}));

const resetPages = (): void => {
  for (const resource of ['users', 'groups', 'roles', 'agents']) {
    mockPages[resource] = {
      actions: {assign: true, create: true, delete: true, edit: true, invite: true, members: true},
      enabled: true,
    };
  }
};

const {flag, listPage} = vi.hoisted(() => {
  const toFlag = (value: unknown): string => (value === true ? 'on' : 'off');

  return {
    flag: toFlag,
    listPage: (name: string) =>
      function MockListPage({canAdd, title}: {canAdd: boolean; title: string}): JSX.Element {
        return (
          <p>
            {name} list {title} add {toFlag(canAdd)}
          </p>
        );
      },
  };
});

vi.mock('@thunderid/manage-users', () => ({
  UserCreateDialog: ({mode}: {mode: string}): JSX.Element => <p>user create {mode}</p>,
  UserDetailsPage: ({deletable, editable}: {deletable: boolean; editable: boolean}): JSX.Element => (
    <p>
      user details edit {flag(editable)} delete {flag(deletable)}
    </p>
  ),
  UsersListPage: ({addAction, title}: {addAction: string | undefined; title: string}): JSX.Element => (
    <p>
      user list {title} add {addAction ?? 'none'}
    </p>
  ),
}));

vi.mock('@thunderid/manage-groups', () => ({
  GroupCreateDialog: (): JSX.Element => <p>group create</p>,
  GroupDetailsPage: ({editable}: {editable: boolean}): JSX.Element => <p>group details members {flag(editable)}</p>,
  GroupsListPage: listPage('group'),
}));

vi.mock('@thunderid/manage-roles', () => ({
  RoleCreateDialog: (): JSX.Element => <p>role create</p>,
  RoleDetailsPage: ({editable}: {editable: boolean}): JSX.Element => <p>role details assign {flag(editable)}</p>,
  RolesListPage: listPage('role'),
}));

vi.mock('@thunderid/manage-agents', () => ({
  AgentCreateDialog: (): JSX.Element => <p>agent create</p>,
  AgentDetailsPage: ({deletable}: {deletable: boolean}): JSX.Element => <p>agent details delete {flag(deletable)}</p>,
  AgentsListPage: listPage('agent'),
}));

vi.mock('@thunderid/react', () => ({
  useUser: () => ({flattenedProfile: {ouName: 'Engineering'}}),
}));

function CurrentPath(): JSX.Element {
  return <output>{useLocation().pathname}</output>;
}

const renderAt = (path: string, element: JSX.Element, routePath: string): void => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path={routePath} element={element} />
        <Route path="*" element={null} />
      </Routes>
      <CurrentPath />
    </MemoryRouter>,
  );
};

describe('Admin App pages', () => {
  beforeEach(() => {
    resetPages();
  });

  it('titles a list with the organization unit the admin manages', () => {
    renderAt('/groups', <ResourceListPage resource="groups" />, '/groups');

    expect(screen.getByText('group list Groups in Engineering add on')).toBeTruthy();
  });

  it('hides the add action when adding is off', () => {
    mockPages.agents.actions = {create: false, delete: false};

    renderAt('/agents', <ResourceListPage resource="agents" />, '/agents');

    expect(screen.getByText('agent list Agents in Engineering add off')).toBeTruthy();
  });

  it('adds people directly when creating is on', () => {
    renderAt('/users', <ResourceListPage resource="users" />, '/users');
    expect(screen.getByText('user list People in Engineering add create')).toBeTruthy();
  });

  it('invites people when only inviting is on', () => {
    mockPages.users.actions = {create: false, invite: true};

    renderAt('/users/new', <ResourceCreatePage resource="users" />, '/users/new');

    expect(screen.getByText('user list People in Engineering add invite')).toBeTruthy();
    expect(screen.getByText('user create invite')).toBeTruthy();
  });

  it('shows the create dialog over the list', () => {
    renderAt('/roles/new', <ResourceCreatePage resource="roles" />, '/roles/new');

    expect(screen.getByText('role list Roles in Engineering add on')).toBeTruthy();
    expect(screen.getByText('role create')).toBeTruthy();
  });

  it('sends the create page back to the list when adding is off', () => {
    mockPages.roles.actions = {create: false};

    renderAt('/roles/new', <ResourceCreatePage resource="roles" />, '/roles/new');

    expect(screen.getByRole('status').textContent).toBe('/roles');
  });

  it('passes the actions the Console allows to the details pages', () => {
    mockPages.users.actions = {delete: false, edit: true};

    renderAt('/users/u-7', <ResourceDetailsPage resource="users" />, '/users/:id');

    expect(screen.getByText('user details edit on delete off')).toBeTruthy();
  });

  it('keeps group members read only when managing members is off', () => {
    mockPages.groups.actions = {members: false};

    renderAt('/groups/g-1', <ResourceDetailsPage resource="groups" />, '/groups/:id');
    expect(screen.getByText('group details members off')).toBeTruthy();
  });

  it('lets admins change role assignments when assigning is on', () => {
    renderAt('/roles/r-1', <ResourceDetailsPage resource="roles" />, '/roles/:id');

    expect(screen.getByText('role details assign on')).toBeTruthy();
  });

  it('sends a page the Console turned off to the first page that is on', () => {
    mockPages.groups.enabled = false;

    renderAt('/groups', <ResourceListPage resource="groups" />, '/groups');

    expect(screen.getByRole('status').textContent).toBe('/users');
  });

  it('explains when no page is on', () => {
    for (const resource of Object.keys(mockPages)) {
      mockPages[resource].enabled = false;
    }

    renderAt('/users', <ResourceListPage resource="users" />, '/users');

    expect(screen.getByRole('heading', {name: 'Nothing to manage yet'})).toBeTruthy();
  });
});
