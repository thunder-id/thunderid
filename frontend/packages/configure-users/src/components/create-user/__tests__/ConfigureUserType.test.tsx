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
import {render, screen, waitFor, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {SchemaInterface} from '../../../models/users';
import ConfigureUserType, {type ConfigureUserTypeProps} from '../ConfigureUserType';

const mockSchemas: SchemaInterface[] = [
  {id: 'schema-1', name: 'Employee', ouId: 'ou-1'},
  {id: 'schema-2', name: 'Contractor', ouId: 'ou-2'},
  {id: 'schema-3', name: 'Vendor', ouId: 'ou-3'},
];

describe('ConfigureUserType', () => {
  const mockOnSchemaChange = vi.fn();
  const mockOnReadyChange = vi.fn();

  const defaultProps: ConfigureUserTypeProps = {
    schemas: mockSchemas,
    selectedSchema: null,
    onSchemaChange: mockOnSchemaChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const renderComponent = (props: Partial<ConfigureUserTypeProps> = {}) =>
    render(<ConfigureUserType {...defaultProps} {...props} />);

  it('renders the component with title and subtitle', () => {
    renderComponent();

    expect(screen.getByRole('heading', {name: 'Select a user type'})).toBeInTheDocument();
    expect(screen.getByText('Choose a user type (schema) for the new user.')).toBeInTheDocument();
  });

  it('renders the user type select field', () => {
    renderComponent();

    expect(screen.getByText('User Type')).toBeInTheDocument();
    expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
  });

  it('renders placeholder when no schema is selected', () => {
    renderComponent();

    expect(screen.getAllByText('Select a user type').length).toBeGreaterThanOrEqual(1);
  });

  it('renders all schema options in the select', async () => {
    const user = userEvent.setup();
    renderComponent();

    const select = screen.getByRole('combobox');
    await user.click(select);

    const listbox = await screen.findByRole('listbox');
    await waitFor(() => {
      expect(within(listbox).getByText('Employee')).toBeInTheDocument();
      expect(within(listbox).getByText('Contractor')).toBeInTheDocument();
      expect(within(listbox).getByText('Vendor')).toBeInTheDocument();
    });
  });

  it('calls onSchemaChange when a schema is selected', async () => {
    const user = userEvent.setup();
    renderComponent();

    const select = screen.getByRole('combobox');
    await user.click(select);

    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByText('Employee'));

    expect(mockOnSchemaChange).toHaveBeenCalledWith(mockSchemas[0]);
  });

  it('calls onSchemaChange when selecting a different schema', async () => {
    const user = userEvent.setup();
    renderComponent({selectedSchema: mockSchemas[0]});

    const select = screen.getByRole('combobox');
    await user.click(select);

    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByText('Contractor'));

    expect(mockOnSchemaChange).toHaveBeenCalledWith(mockSchemas[1]);
  });

  it('displays the selected schema name', () => {
    renderComponent({selectedSchema: mockSchemas[0]});

    expect(screen.getByText('Employee')).toBeInTheDocument();
  });

  describe('onReadyChange callback', () => {
    it('calls onReadyChange with true when a schema is selected', () => {
      renderComponent({
        selectedSchema: mockSchemas[0],
        onReadyChange: mockOnReadyChange,
      });

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('calls onReadyChange with false when no schema is selected', () => {
      renderComponent({
        selectedSchema: null,
        onReadyChange: mockOnReadyChange,
      });

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('does not crash when onReadyChange is undefined', () => {
      expect(() => {
        renderComponent({selectedSchema: mockSchemas[0], onReadyChange: undefined});
      }).not.toThrow();
    });

    it('calls onReadyChange when selectedSchema transitions from null to non-null', () => {
      const {rerender} = render(
        <ConfigureUserType {...defaultProps} selectedSchema={null} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
      mockOnReadyChange.mockClear();

      rerender(
        <ConfigureUserType {...defaultProps} selectedSchema={mockSchemas[0]} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });
  });

  it('handles empty schemas list', () => {
    renderComponent({schemas: []});

    expect(screen.getByTestId('configure-user-type')).toBeInTheDocument();
  });
});
