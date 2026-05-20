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
import {render, screen, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import AllowedUserTypesSection from '../AllowedUserTypesSection';

const {mockUseGetUserTypes} = vi.hoisted(() => ({
  mockUseGetUserTypes: vi.fn(),
}));

vi.mock('../../../../../user-types/api/useGetUserTypes', () => ({
  default: (): unknown => mockUseGetUserTypes() as unknown,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('AllowedUserTypesSection', () => {
  const mockOnFieldChange = vi.fn();

  const agent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test Agent',
    allowedUserTypes: ['employee'],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUserTypes.mockReturnValue({
      data: {
        types: [
          {id: 'ut-1', name: 'employee'},
          {id: 'ut-2', name: 'customer'},
        ],
      },
      isLoading: false,
    });
  });

  it('renders the section title and description', () => {
    render(<AllowedUserTypesSection agent={agent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('Allowed User Types')).toBeInTheDocument();
    expect(
      screen.getByText('Restrict which user types can authenticate or register through this agent.'),
    ).toBeInTheDocument();
  });

  it('renders existing allowedUserTypes as chips', () => {
    render(<AllowedUserTypesSection agent={agent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('employee')).toBeInTheDocument();
  });

  it('prioritizes editedAgent.allowedUserTypes over agent.allowedUserTypes', () => {
    render(
      <AllowedUserTypesSection
        agent={agent}
        editedAgent={{allowedUserTypes: ['customer']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByText('customer')).toBeInTheDocument();
    expect(screen.queryByText('employee')).not.toBeInTheDocument();
  });

  it('falls back to an empty array when neither value is set', () => {
    render(
      <AllowedUserTypesSection
        agent={{...agent, allowedUserTypes: undefined}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.queryByText('employee')).not.toBeInTheDocument();
  });

  it('calls onFieldChange when a user type is added', async () => {
    const user = userEvent.setup();
    render(
      <AllowedUserTypesSection
        agent={{...agent, allowedUserTypes: []}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    const combobox = screen.getByRole('combobox');
    await user.click(combobox);

    const listbox = screen.getByRole('listbox');
    await user.click(within(listbox).getByText('employee'));

    expect(mockOnFieldChange).toHaveBeenCalledWith('allowedUserTypes', ['employee']);
  });

  it('renders user-type options from the API response', async () => {
    const user = userEvent.setup();
    render(
      <AllowedUserTypesSection
        agent={{...agent, allowedUserTypes: []}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('combobox'));

    const listbox = screen.getByRole('listbox');
    expect(within(listbox).getByText('employee')).toBeInTheDocument();
    expect(within(listbox).getByText('customer')).toBeInTheDocument();
  });

  it('handles missing user-type schemas gracefully', () => {
    mockUseGetUserTypes.mockReturnValueOnce({data: {types: undefined}, isLoading: false});
    render(<AllowedUserTypesSection agent={agent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('employee')).toBeInTheDocument();
  });
});
