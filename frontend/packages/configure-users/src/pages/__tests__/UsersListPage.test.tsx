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

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UsersListPage from '../UsersListPage';

const mockNavigate = vi.fn();
const mockLoggerError = vi.fn();

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    info: vi.fn(),
    error: mockLoggerError,
    debug: vi.fn(),
    warn: vi.fn(),
  }),
}));

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock the UsersList component
vi.mock('@/components/UsersList', () => ({
  default: () => <div data-testid="users-list">Users List Component</div>,
}));

describe('UsersListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLoggerError.mockReset();
  });

  it('renders page title', () => {
    render(<UsersListPage />);

    expect(screen.getByText('User Management')).toBeInTheDocument();
  });

  it('renders page description', () => {
    render(<UsersListPage />);

    expect(screen.getByText('Manage users, roles, and permissions across your organization')).toBeInTheDocument();
  });

  it('renders create user button', () => {
    render(<UsersListPage />);

    const createButton = screen.getByRole('button', {name: /add user/i});
    expect(createButton).toBeInTheDocument();
  });

  it('renders search input', () => {
    render(<UsersListPage />);

    const searchInput = screen.getByPlaceholderText('Search users...');
    expect(searchInput).toBeInTheDocument();
  });

  it('renders search icon', () => {
    const {container} = render(<UsersListPage />);

    const searchIcon = container.querySelector('svg');
    expect(searchIcon).toBeInTheDocument();
  });

  it('allows typing in search input', async () => {
    const user = userEvent.setup();
    render(<UsersListPage />);

    const searchInput = screen.getByPlaceholderText('Search users...');
    await user.type(searchInput, 'john doe');

    expect(searchInput).toHaveValue('john doe');
  });

  it('navigates to add user flow when create button is clicked', async () => {
    const user = userEvent.setup();
    render(<UsersListPage />);

    const createButton = screen.getByRole('button', {name: /add user/i});
    await user.click(createButton);

    expect(mockNavigate).toHaveBeenCalledWith('/users/invite');
  });

  it('renders UsersList component', () => {
    render(<UsersListPage />);

    expect(screen.getByTestId('users-list')).toBeInTheDocument();
  });

  it('renders plus icon in create user button', () => {
    render(<UsersListPage />);

    const createButton = screen.getByRole('button', {name: /add user/i});
    const icon = createButton.querySelector('svg');
    expect(icon).toBeInTheDocument();
  });

  it('has correct heading level', () => {
    render(<UsersListPage />);

    const heading = screen.getByRole('heading', {level: 1, name: /user management/i});
    expect(heading).toBeInTheDocument();
  });

  it('create user button has contained variant', () => {
    render(<UsersListPage />);

    const createButton = screen.getByRole('button', {name: /add user/i});
    expect(createButton).toHaveClass('MuiButton-contained');
  });

  it('handles navigation error gracefully', async () => {
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    const user = userEvent.setup();
    render(<UsersListPage />);

    const createButton = screen.getByRole('button', {name: /add user/i});
    await user.click(createButton);

    expect(mockNavigate).toHaveBeenCalledWith('/users/invite');

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to add user page',
        expect.objectContaining({error: navigationError}),
      );
    });
  });
});
