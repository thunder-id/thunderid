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
import {render, screen, within, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ConfigureOwner, {type ConfigureOwnerProps} from '../ConfigureOwner';

const {mockCurrentUser, mockUseGetUsers} = vi.hoisted(() => ({
  mockCurrentUser: {id: 'current-user-id'},
  mockUseGetUsers: vi.fn(),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({user: mockCurrentUser}),
}));

vi.mock('@thunderid/configure-users', () => ({
  useGetUsers: (...args: unknown[]): unknown => mockUseGetUsers(...args) as unknown,
}));

describe('ConfigureOwner', () => {
  const mockOnOwnerIdChange = vi.fn();

  const defaultProps: ConfigureOwnerProps = {
    selectedOwnerId: null,
    onOwnerIdChange: mockOnOwnerIdChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUsers.mockReturnValue({
      data: {
        users: [
          {id: 'user-1', display: 'Alice', attributes: {username: 'alice', email: 'alice@example.com'}},
          {id: 'user-2', attributes: {username: 'bob', email: 'bob@example.com'}},
          {id: 'user-3', attributes: {email: 'charlie@example.com'}},
          {id: 'user-4', attributes: {}},
        ],
      },
      isLoading: false,
    });
  });

  it('renders title and subtitle', () => {
    render(<ConfigureOwner {...defaultProps} />);

    expect(screen.getByRole('heading', {level: 1, name: 'Owner'})).toBeInTheDocument();
    expect(screen.getByText('Choose the user that owns this agent.')).toBeInTheDocument();
  });

  it('defaults to current user when nothing is selected', () => {
    render(<ConfigureOwner {...defaultProps} />);

    expect(mockOnOwnerIdChange).toHaveBeenCalledWith('current-user-id');
  });

  it('does not override an existing selection', () => {
    render(<ConfigureOwner {...defaultProps} selectedOwnerId="user-2" />);

    expect(mockOnOwnerIdChange).not.toHaveBeenCalled();
  });

  it('reports ready=true on mount', () => {
    const onReadyChange = vi.fn();
    render(<ConfigureOwner {...defaultProps} onReadyChange={onReadyChange} />);

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('shows a loading placeholder while users are loading', () => {
    mockUseGetUsers.mockReturnValueOnce({data: undefined, isLoading: true});
    render(<ConfigureOwner {...defaultProps} />);

    // The Select renders a disabled MenuItem with the loading text
    expect(screen.getByText(/Loading…|Loading\.\.\./)).toBeInTheDocument();
  });

  it('calls onOwnerIdChange when a user is picked', async () => {
    const user = userEvent.setup();
    render(<ConfigureOwner {...defaultProps} selectedOwnerId="user-1" />);

    const select = screen.getByRole('combobox');
    await user.click(select);

    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByText('bob'));

    await waitFor(() => {
      expect(mockOnOwnerIdChange).toHaveBeenCalledWith('user-2');
    });
  });

  it('falls back to email when username is missing in attributes', () => {
    render(<ConfigureOwner {...defaultProps} selectedOwnerId="user-3" />);

    expect(screen.getByText('charlie@example.com')).toBeInTheDocument();
  });

  it('falls back to id when both username and email are missing', () => {
    render(<ConfigureOwner {...defaultProps} selectedOwnerId="user-4" />);

    expect(screen.getByText('user-4')).toBeInTheDocument();
  });

  it('uses display field when present', () => {
    render(<ConfigureOwner {...defaultProps} selectedOwnerId="user-1" />);

    expect(screen.getByText('Alice')).toBeInTheDocument();
  });
});
