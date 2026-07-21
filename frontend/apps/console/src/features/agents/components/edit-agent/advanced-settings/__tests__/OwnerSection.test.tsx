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
import type {Agent} from '../../../../models/agent';
import OwnerSection from '../OwnerSection';

const {mockUseGetUsers} = vi.hoisted(() => ({
  mockUseGetUsers: vi.fn(),
}));

vi.mock('@thunderid/configure-users', () => ({
  useGetUsers: (...args: unknown[]): unknown => mockUseGetUsers(...args) as unknown,
}));

describe('OwnerSection', () => {
  const mockOnFieldChange = vi.fn();

  const mockAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test Agent',
    owner: 'user-1',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUsers.mockReturnValue({
      data: {
        users: [
          {id: 'user-1', display: 'Alice'},
          {id: 'user-2', display: 'Bob'},
        ],
      },
      isLoading: false,
    });
  });

  it('renders the resolved owner label', () => {
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('shows a placeholder when no owner is assigned', () => {
    render(
      <OwnerSection agent={{...mockAgent, owner: undefined}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByText('No owner assigned')).toBeInTheDocument();
  });

  it('prefers the edited owner over the saved one', () => {
    render(<OwnerSection agent={mockAgent} editedAgent={{owner: 'user-2'}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('Bob')).toBeInTheDocument();
  });

  it('opens a picker and calls onFieldChange when a new owner is selected', async () => {
    const user = userEvent.setup();
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    await user.click(screen.getByRole('combobox'));

    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByText('Bob'));

    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenCalledWith('owner', 'user-2');
    });
  });

  it('disables the picker for read-only agents', () => {
    render(
      <OwnerSection agent={{...mockAgent, isReadOnly: true}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
  });

  it('falls back to the username attribute when no display name is set', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {username: 'alice.doe'}}]},
      isLoading: false,
    });

    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('alice.doe')).toBeInTheDocument();
  });

  it('falls back to the email attribute when there is no display name or username', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {email: 'alice@example.com'}}]},
      isLoading: false,
    });

    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  });

  it('falls back to the user id when no display name, username, or email is available', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {}}]},
      isLoading: false,
    });

    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
  });
});
