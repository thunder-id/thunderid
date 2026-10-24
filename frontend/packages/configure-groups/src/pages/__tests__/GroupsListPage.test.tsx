// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import GroupsListPage from '../GroupsListPage';

vi.mock('../../components/GroupsList', () => ({
  default: () => <div data-testid="groups-list">GroupsList Mock</div>,
}));

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe('GroupsListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render page title and subtitle', () => {
    renderWithProviders(<GroupsListPage />);

    expect(screen.getByText('Groups')).toBeInTheDocument();
    expect(screen.getByText('Manage groups and their members across organization units')).toBeInTheDocument();
  });

  it('should set the document title', async () => {
    renderWithProviders(<GroupsListPage />);

    await waitFor(() => expect(document.title).toBe('Groups | ThunderID'));
  });

  it('should render add group button', () => {
    renderWithProviders(<GroupsListPage />);

    expect(screen.getByText('Add group')).toBeInTheDocument();
  });

  it('should navigate to create page on add group click', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupsListPage />);

    await user.click(screen.getByText('Add group'));

    expect(mockNavigate).toHaveBeenCalledWith('/groups/create');
  });

  it('should render GroupsList component', () => {
    renderWithProviders(<GroupsListPage />);

    expect(screen.getByTestId('groups-list')).toBeInTheDocument();
  });
});
