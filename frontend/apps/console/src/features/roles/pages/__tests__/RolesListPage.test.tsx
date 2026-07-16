/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import RolesListPage from '../RolesListPage';

// Mock dependencies
vi.mock('../../components/RolesList', () => ({
  default: () => <div data-testid="roles-list">Roles List</div>,
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'roles:listing.title': 'Roles',
        'roles:listing.subtitle': 'Manage roles and permissions',
        'roles:listing.addRole': 'Add Role',
        'roles:listing.search.placeholder': 'Search roles...',
      };
      return translations[key] || key;
    },
  }),
}));

const {useNavigate} = await import('react-router');

describe('RolesListPage', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockNavigate = vi.fn();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render the RolesList component', () => {
    render(<RolesListPage />);

    expect(screen.getByTestId('roles-list')).toBeInTheDocument();
  });

  it('should render the Add Role button', () => {
    render(<RolesListPage />);

    expect(screen.getByRole('button', {name: /add role/i})).toBeInTheDocument();
  });

  it('should navigate to create page when Add Role button is clicked', async () => {
    const user = userEvent.setup();
    render(<RolesListPage />);

    const addButton = screen.getByRole('button', {name: /add role/i});
    await user.click(addButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/create');
    });
  });
});
