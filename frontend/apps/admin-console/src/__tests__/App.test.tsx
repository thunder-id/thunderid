// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import App from '../App';

vi.mock('@thunderid/react-router', () => ({
  ProtectedRoute: ({children}: {children: React.ReactNode}) => <div data-testid="protected-route">{children}</div>,
}));

vi.mock('@thunderid/configure-users', () => ({
  UserAddPage: () => <div data-testid="user-add-page">User Add Page</div>,
  UserEditPage: () => <div data-testid="user-edit-page">User Edit Page</div>,
  UsersListPage: () => <div data-testid="users-list-page">Users List Page</div>,
}));

vi.mock('@thunderid/configure-agents', () => ({
  AgentsListPage: () => <div data-testid="agents-list-page">AgentsListPage</div>,
  AgentEditPage: () => <div data-testid="agent-edit-page">AgentEditPage</div>,
  AgentOnboardPage: () => <div data-testid="agent-onboard-page">AgentOnboardPage</div>,
}));

vi.mock('@thunderid/configure-groups', () => ({
  GroupCreateProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
  GroupsListPage: () => <div data-testid="groups-list-page">GroupsListPage</div>,
  GroupEditPage: () => <div data-testid="group-edit-page">GroupEditPage</div>,
  CreateGroupPage: () => <div data-testid="create-group-page">CreateGroupPage</div>,
}));

vi.mock('@thunderid/configure-roles', () => ({
  RoleCreateProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
  RolesListPage: () => <div data-testid="roles-list-page">RolesListPage</div>,
  RoleEditPage: () => <div data-testid="role-edit-page">RoleEditPage</div>,
  CreateRolePage: () => <div data-testid="create-role-page">CreateRolePage</div>,
}));

vi.mock('@thunderid/configure-agent-types', () => ({
  ViewAgentTypePage: () => <div data-testid="view-agent-type-page">View Agent Type Page</div>,
}));

vi.mock('../lib/monaco-setup', () => ({}));

vi.mock('../layouts/DashboardLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('../layouts/FullScreenLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

describe('App', () => {
  afterEach(() => {
    window.history.pushState({}, '', '/');
  });

  it('renders without crashing', () => {
    const {container} = render(<App />);
    expect(container).toBeInTheDocument();
  });

  it('redirects the root path to the users list', async () => {
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('users-list-page')).toBeInTheDocument();
    });
  });

  it('loads UserEditPage lazily at /users/:userId', async () => {
    window.history.pushState({}, '', '/users/user-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('user-edit-page')).toBeInTheDocument();
    });
  });

  it('loads UserAddPage lazily at /users/add', async () => {
    window.history.pushState({}, '', '/users/add');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('user-add-page')).toBeInTheDocument();
    });
  });

  it('loads AgentsListPage lazily at /agents', async () => {
    window.history.pushState({}, '', '/agents');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('agents-list-page')).toBeInTheDocument();
    });
  });

  it('loads AgentEditPage lazily via the monaco-setup chain at /agents/a-1', async () => {
    window.history.pushState({}, '', '/agents/a-1');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('agent-edit-page')).toBeInTheDocument();
    });
  });

  it('loads AgentOnboardPage lazily at /agents/create', async () => {
    window.history.pushState({}, '', '/agents/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('agent-onboard-page')).toBeInTheDocument();
    });
  });

  it('loads GroupsListPage lazily at /groups', async () => {
    window.history.pushState({}, '', '/groups');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('groups-list-page')).toBeInTheDocument();
    });
  });

  it('loads GroupEditPage lazily at /groups/g-1', async () => {
    window.history.pushState({}, '', '/groups/g-1');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('group-edit-page')).toBeInTheDocument();
    });
  });

  it('loads CreateGroupPage lazily at /groups/create', async () => {
    window.history.pushState({}, '', '/groups/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-group-page')).toBeInTheDocument();
    });
  });

  it('loads RolesListPage lazily at /roles', async () => {
    window.history.pushState({}, '', '/roles');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('roles-list-page')).toBeInTheDocument();
    });
  });

  it('loads RoleEditPage lazily at /roles/r-1', async () => {
    window.history.pushState({}, '', '/roles/r-1');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('role-edit-page')).toBeInTheDocument();
    });
  });

  it('loads CreateRolePage lazily at /roles/create', async () => {
    window.history.pushState({}, '', '/roles/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-role-page')).toBeInTheDocument();
    });
  });

  it('loads ViewAgentTypePage lazily at /agent-types/:id', async () => {
    window.history.pushState({}, '', '/agent-types/at-1');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('view-agent-type-page')).toBeInTheDocument();
    });
  });
});
