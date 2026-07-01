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
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import ConfigureName from '../ConfigureName';
import type {ConfigureNameProps} from '../ConfigureName';

vi.mock('@thunderid/utils');

const mockSuggestions = ['Alpha PID', 'Beta PID', 'Gamma PID'];
const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

describe('ConfigureName', () => {
  const mockOnNameChange = vi.fn();
  const mockOnHandleChange = vi.fn();
  const mockOnHandleEditedChange = vi.fn();

  const defaultProps: ConfigureNameProps = {
    name: '',
    handle: '',
    handleEdited: false,
    onNameChange: mockOnNameChange,
    onHandleChange: mockOnHandleChange,
    onHandleEditedChange: mockOnHandleEditedChange,
  };

  const renderComponent = (props = defaultProps) => render(<ConfigureName {...props} />);

  beforeEach(() => {
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue(mockSuggestions);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render the component with test id', () => {
    renderComponent();

    expect(screen.getByTestId('configure-name')).toBeInTheDocument();
  });

  it('should render name text field', () => {
    renderComponent();

    expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
  });

  it('should render handle text field', () => {
    renderComponent();

    expect(screen.getByLabelText(/handle/i)).toBeInTheDocument();
  });

  it('should display current name value', () => {
    renderComponent({...defaultProps, name: 'EUDI Wallet PID'});

    const nameInput = screen.getByLabelText(/name/i);
    expect(nameInput).toHaveValue('EUDI Wallet PID');
  });

  it('should display current handle value', () => {
    renderComponent({...defaultProps, handle: 'eudi-wallet-pid'});

    const handleInput = screen.getByLabelText(/handle/i);
    expect(handleInput).toHaveValue('eudi-wallet-pid');
  });

  it('should call onNameChange when typing in name input', async () => {
    const user = userEvent.setup();
    renderComponent();

    const nameInput = screen.getByLabelText(/name/i);
    await user.type(nameInput, 'A');

    expect(mockOnNameChange).toHaveBeenCalledWith('A');
  });

  it('should derive and set the handle from the name when handle has not been manually edited', async () => {
    const user = userEvent.setup();
    renderComponent();

    const nameInput = screen.getByLabelText(/name/i);
    await user.type(nameInput, 'A');

    expect(mockOnHandleChange).toHaveBeenCalledWith('a');
  });

  it('should not derive the handle from the name once the handle has been manually edited', async () => {
    const user = userEvent.setup();
    renderComponent({...defaultProps, handleEdited: true});

    const nameInput = screen.getByLabelText(/name/i);
    await user.type(nameInput, 'A');

    expect(mockOnHandleChange).not.toHaveBeenCalled();
  });

  it('should mark the handle as manually edited and sanitize input when typing in the handle field', async () => {
    const user = userEvent.setup();
    renderComponent();

    const handleInput = screen.getByLabelText(/handle/i);
    await user.type(handleInput, 'A');

    expect(mockOnHandleEditedChange).toHaveBeenCalledWith(true);
    expect(mockOnHandleChange).toHaveBeenCalledWith('a');
  });

  it('should strip disallowed characters when typing in the handle field', async () => {
    const user = userEvent.setup();
    renderComponent();

    const handleInput = screen.getByLabelText(/handle/i);
    await user.type(handleInput, '!');

    expect(mockOnHandleChange).toHaveBeenCalledWith('');
  });

  it('should render name suggestions', () => {
    renderComponent();

    mockSuggestions.forEach((suggestion) => {
      expect(screen.getByText(suggestion)).toBeInTheDocument();
    });
  });

  it('should call onNameChange, onHandleChange and reset handleEdited when clicking a suggestion chip', async () => {
    const user = userEvent.setup();
    renderComponent({...defaultProps, handleEdited: true});

    const suggestionChip = screen.getByText('Alpha PID');
    await user.click(suggestionChip);

    expect(mockOnNameChange).toHaveBeenCalledWith('Alpha PID');
    expect(mockOnHandleChange).toHaveBeenCalledWith('alpha-pid');
    expect(mockOnHandleEditedChange).toHaveBeenCalledWith(false);
  });

  it('should generate suggestions only once on mount', () => {
    const {rerender} = renderComponent();

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);

    rerender(<ConfigureName {...defaultProps} name="Updated" />);

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);
  });

  it('should update input value when name prop changes', () => {
    const {rerender} = renderComponent({...defaultProps, name: 'Initial Name'});

    let nameInput = screen.getByLabelText(/name/i);
    expect(nameInput).toHaveValue('Initial Name');

    rerender(<ConfigureName {...defaultProps} name="Updated Name" />);

    nameInput = screen.getByLabelText(/name/i);
    expect(nameInput).toHaveValue('Updated Name');
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with true when name and handle are both non-empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({
        ...defaultProps,
        name: 'EUDI Wallet PID',
        handle: 'eudi-wallet-pid',
        onReadyChange: mockOnReadyChange,
      });

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when name is empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: '', handle: 'eudi-wallet-pid', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with false when handle is empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: 'EUDI Wallet PID', handle: '', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with false when name contains only whitespace', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: '   ', handle: 'eudi-wallet-pid', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should not crash when onReadyChange is undefined', () => {
      expect(() => {
        renderComponent({...defaultProps, onReadyChange: undefined});
      }).not.toThrow();
    });
  });
});
