// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

  it('should set the document title', async () => {
    render(<RolesListPage />);

    await waitFor(() => expect(document.title).toBe('Roles | ThunderID'));
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
