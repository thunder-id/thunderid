/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen, waitFor} from '@testing-library/react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import App from '../App';

vi.mock('@thunderid/react-router', () => ({
  ProtectedRoute: ({children}: {children: React.ReactNode}) => <div data-testid="protected-route">{children}</div>,
}));

vi.mock('@thunderid/configure-translations', () => ({
  TranslationCreateProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
  TranslationCreatePage: () => <div data-testid="translation-create-page" />,
  TranslationsEditPage: () => <div data-testid="translations-edit-page" />,
  TranslationsListPage: () => <div data-testid="translations-list-page" />,
}));

vi.mock('../lib/monaco-setup', () => ({}));

vi.mock('../features/home/pages/HomePage', () => ({
  default: () => <div data-testid="home-page" />,
}));

vi.mock('../features/users/pages/UsersListPage', () => ({
  default: () => <div data-testid="users-list-page">Users List Page</div>,
}));

vi.mock('../features/users/pages/UserCreatePage', () => ({
  default: () => <div data-testid="create-user-page">Create User Page</div>,
}));

vi.mock('../features/users/pages/UserEditPage', () => ({
  default: () => <div data-testid="user-edit-page">User Edit Page</div>,
}));

vi.mock('../features/user-types/pages/UserTypesListPage', () => ({
  default: () => <div data-testid="user-types-list-page">User Types List Page</div>,
}));

vi.mock('../features/user-types/pages/CreateUserTypePage', () => ({
  default: () => <div data-testid="create-user-type-page">Create User Type Page</div>,
}));

vi.mock('../features/user-types/pages/ViewUserTypePage', () => ({
  default: () => <div data-testid="view-user-type-page">View User Type Page</div>,
}));

vi.mock('../features/integrations/pages/IntegrationsPage', () => ({
  default: () => <div data-testid="integrations-page">Integrations Page</div>,
}));

vi.mock('../features/applications/pages/ApplicationsListPage', () => ({
  default: () => <div data-testid="applications-list-page">Applications List Page</div>,
}));

vi.mock('../features/applications/pages/ApplicationCreatePage', () => ({
  default: () => <div data-testid="application-create-page">Application Create Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers', () => ({
  ResourceServersListPage: () => <div data-testid="resource-servers-list-page">Resource Servers List Page</div>,
  ResourceServerEditPage: () => <div data-testid="resource-server-edit-page">Resource Server Edit Page</div>,
  CreateResourceServerPage: () => <div data-testid="create-resource-server-page">Create Resource Server Page</div>,
}));

vi.mock('../layouts/DashboardLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('../layouts/FullScreenLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('../features/welcome/components/WelcomeRedirect', () => ({
  default: () => null,
}));

describe('App', () => {
  afterEach(() => {
    window.history.pushState({}, '', '/');
  });

  it('renders without crashing', () => {
    const {container} = render(<App />);
    expect(container).toBeInTheDocument();
  });

  it('loads TranslationsEditPage lazily via the monaco-setup chain', async () => {
    window.history.pushState({}, '', '/translations/en');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('translations-edit-page')).toBeInTheDocument();
    });
  });

  it('loads ResourceServersListPage lazily at /resource-servers', async () => {
    window.history.pushState({}, '', '/resource-servers');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('resource-servers-list-page')).toBeInTheDocument();
    });
  });

  it('loads ResourceServerEditPage lazily at /resource-servers/:id', async () => {
    window.history.pushState({}, '', '/resource-servers/rs-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('resource-server-edit-page')).toBeInTheDocument();
    });
  });

  it('loads CreateResourceServerPage lazily at /resource-servers/create', async () => {
    window.history.pushState({}, '', '/resource-servers/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-resource-server-page')).toBeInTheDocument();
    });
  });
});
