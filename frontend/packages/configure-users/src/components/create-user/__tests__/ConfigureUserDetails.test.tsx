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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ApiUserType} from '../../../models/users';
import ConfigureUserDetails, {type ConfigureUserDetailsProps} from '../ConfigureUserDetails';

const mockSchema: ApiUserType = {
  id: 'schema-1',
  name: 'Employee',
  schema: {
    username: {
      type: 'string',
      required: true,
    },
    age: {
      type: 'number',
      required: false,
    },
    active: {
      type: 'boolean',
      required: false,
    },
  },
};

describe('ConfigureUserDetails', () => {
  const mockOnFormValuesChange = vi.fn();
  const mockOnReadyChange = vi.fn();

  const defaultProps: ConfigureUserDetailsProps = {
    schema: mockSchema,
    defaultValues: {},
    onFormValuesChange: mockOnFormValuesChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const renderComponent = (props: Partial<ConfigureUserDetailsProps> = {}) =>
    render(<ConfigureUserDetails {...defaultProps} {...props} />);

  it('renders the component with title and subtitle', () => {
    renderComponent();

    expect(screen.getByText('Enter user details')).toBeInTheDocument();
    expect(screen.getByText('Fill in the required information for the new user.')).toBeInTheDocument();
  });

  it('renders the data-testid attribute', () => {
    renderComponent();

    expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
  });

  it('renders string fields from the schema', () => {
    renderComponent();

    expect(screen.getByPlaceholderText(/enter username/i)).toBeInTheDocument();
  });

  it('renders number fields from the schema', () => {
    renderComponent();

    expect(screen.getByPlaceholderText(/enter age/i)).toBeInTheDocument();
  });

  it('renders boolean fields from the schema', () => {
    renderComponent();

    expect(screen.getByRole('checkbox')).toBeInTheDocument();
  });

  it('calls onFormValuesChange when form values change', async () => {
    const user = userEvent.setup();
    renderComponent();

    const usernameInput = screen.getByPlaceholderText(/enter username/i);
    await user.type(usernameInput, 'john');

    await waitFor(() => {
      expect(mockOnFormValuesChange).toHaveBeenCalled();
      const lastCall = mockOnFormValuesChange.mock.calls[mockOnFormValuesChange.mock.calls.length - 1][0] as Record<
        string,
        unknown
      >;
      expect(lastCall).toHaveProperty('username', 'john');
    });
  });

  it('renders with default values pre-filled', () => {
    renderComponent({
      defaultValues: {username: 'existing_user', age: 25},
    });

    expect(screen.getByPlaceholderText(/enter username/i)).toHaveValue('existing_user');
  });

  describe('onReadyChange callback', () => {
    it('calls onReadyChange with false when required fields are empty', () => {
      renderComponent({onReadyChange: mockOnReadyChange});

      // username is required and starts empty, so form is not valid
      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('calls onReadyChange with true when required fields are filled', async () => {
      const user = userEvent.setup();
      renderComponent({onReadyChange: mockOnReadyChange});

      const usernameInput = screen.getByPlaceholderText(/enter username/i);
      await user.type(usernameInput, 'john');

      await waitFor(() => {
        expect(mockOnReadyChange).toHaveBeenCalledWith(true);
      });
    });

    it('does not crash when onReadyChange is undefined', () => {
      expect(() => {
        renderComponent({onReadyChange: undefined});
      }).not.toThrow();
    });
  });

  it('renders credential fields as password inputs with toggle visibility', () => {
    const schemaWithCredential: ApiUserType = {
      id: 'schema-cred',
      name: 'Employee',
      schema: {
        username: {
          type: 'string',
          required: true,
        },
        password: {
          type: 'string',
          required: true,
          credential: true,
        },
      },
    };

    renderComponent({schema: schemaWithCredential});

    const passwordInput = screen.getByPlaceholderText(/enter password/i);
    expect(passwordInput).toHaveAttribute('type', 'password');
    expect(screen.getByLabelText('show password')).toBeInTheDocument();

    const usernameInput = screen.getByPlaceholderText(/enter username/i);
    expect(usernameInput).toHaveAttribute('type', 'text');
  });

  it('handles schema with no fields', () => {
    const emptySchema: ApiUserType = {
      id: 'schema-empty',
      name: 'Empty',
      schema: {},
    };

    renderComponent({schema: emptySchema});

    expect(screen.getByTestId('configure-user-details')).toBeInTheDocument();
  });
});
