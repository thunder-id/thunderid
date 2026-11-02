// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserTypesListPage from '../UserTypesListPage';

const mockNavigate = vi.fn();
const mockLoggerError = vi.fn();

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

// Mock the UserTypesList component
vi.mock('../../components/UserTypesList', () => ({
  default: () => (
    <div data-testid="user-types-list">
      <span>Organization unit: Root Organization</span>
    </div>
  ),
}));

describe('UserTypesListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLoggerError.mockReset();
  });

  it('renders page title', () => {
    render(<UserTypesListPage />);

    expect(screen.getByText('User types')).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    render(<UserTypesListPage />);

    await waitFor(() => expect(document.title).toBe('User types | ThunderID'));
  });

  it('renders page description', () => {
    render(<UserTypesListPage />);

    expect(screen.getByText('Define and manage user types with custom schemas')).toBeInTheDocument();
  });

  it('renders create user type button', () => {
    render(<UserTypesListPage />);

    const createButton = screen.getByRole('button', {name: /create user type/i});
    expect(createButton).toBeInTheDocument();
  });

  it('renders UserTypesList component', () => {
    render(<UserTypesListPage />);

    expect(screen.getByTestId('user-types-list')).toBeInTheDocument();
  });

  it('shows organization unit name within the list component', () => {
    render(<UserTypesListPage />);

    expect(screen.getByText('Organization unit: Root Organization')).toBeInTheDocument();
  });

  it('navigates to create page when create button is clicked', async () => {
    const user = userEvent.setup();
    render(<UserTypesListPage />);

    const createButton = screen.getByRole('button', {name: /create user type/i});
    await user.click(createButton);

    expect(mockNavigate).toHaveBeenCalledWith('/user-types/create');
  });

  it('handles navigation error when create button is clicked and logs error', async () => {
    const user = userEvent.setup();
    const testError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(testError);

    render(<UserTypesListPage />);

    const createButton = screen.getByRole('button', {name: /create user type/i});
    await user.click(createButton);

    expect(mockNavigate).toHaveBeenCalledWith('/user-types/create');

    // Wait for the error to be logged
    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to create user type page',
        expect.objectContaining({error: testError}),
      );
    });
  });
});
