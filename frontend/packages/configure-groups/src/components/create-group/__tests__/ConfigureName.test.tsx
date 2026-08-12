// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ConfigureName, {type ConfigureNameProps} from '../ConfigureName';

vi.mock('@thunderid/utils');

const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

describe('ConfigureName', () => {
  const mockOnNameChange = vi.fn();
  const mockSuggestions = ['Brave Tigers Squad', 'Crimson Hawks Team', 'Golden Wolves Pack', 'Silver Eagles Crew'];

  const defaultProps: ConfigureNameProps = {
    name: '',
    onNameChange: mockOnNameChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue(mockSuggestions);
  });

  const renderComponent = (props: Partial<ConfigureNameProps> = {}) =>
    render(<ConfigureName {...defaultProps} {...props} />);

  it('should render the component with test id', () => {
    renderComponent();

    expect(screen.getByTestId('configure-name')).toBeInTheDocument();
  });

  it('should render the title heading', () => {
    renderComponent();

    expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
  });

  it('should render the text field with correct label', () => {
    renderComponent();

    expect(screen.getByText('Group Name')).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toBeInTheDocument();
  });

  it('should display the current name value', () => {
    renderComponent({name: 'My Test Group'});

    const input = screen.getByRole('textbox');
    expect(input).toHaveValue('My Test Group');
  });

  it('should call onNameChange when typing in the input', async () => {
    const user = userEvent.setup();
    renderComponent();

    const input = screen.getByRole('textbox');
    await user.type(input, 'New Group');

    expect(mockOnNameChange).toHaveBeenCalledTimes(9); // Once per character
  });

  it('should render a name suggestion', () => {
    renderComponent();

    expect(screen.getByText('Brave Tigers Squad')).toBeInTheDocument();
  });

  it('should display the suggestion prefix label', () => {
    renderComponent();

    expect(screen.getByText('Need inspiration? How about')).toBeInTheDocument();
  });

  it('should call onNameChange when clicking the suggestion', async () => {
    const user = userEvent.setup();
    renderComponent();

    const suggestion = screen.getByText('Brave Tigers Squad');
    await user.click(suggestion);

    expect(mockOnNameChange).toHaveBeenCalledWith('Brave Tigers Squad');
  });

  it('should render the suggestion as clickable', () => {
    renderComponent();

    const suggestion = screen.getByText('Brave Tigers Squad');
    expect(suggestion).toHaveAttribute('role', 'button');
  });

  it('should generate suggestions only once on mount', () => {
    const {rerender} = renderComponent();

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);

    rerender(<ConfigureName {...defaultProps} name="Updated Name" />);

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);
  });

  it('should display placeholder text', () => {
    renderComponent();

    const input = screen.getByRole('textbox');
    expect(input).toHaveAttribute('placeholder');
  });

  it('should render required field indicator', () => {
    renderComponent();

    const label = screen.getByText('Group Name');
    const labelElement = label.closest('label');
    expect(labelElement).toHaveClass('Mui-required');
  });

  it('should allow clearing the input', async () => {
    const user = userEvent.setup();
    renderComponent({name: 'Some Group'});

    const input = screen.getByRole('textbox');
    await user.clear(input);

    expect(mockOnNameChange).toHaveBeenCalledWith('');
  });

  it('should request a new suggestion when the shuffle button is clicked', async () => {
    const user = userEvent.setup();
    vi.mocked(generateRandomHumanReadableIdentifiers)
      .mockReturnValueOnce(['Brave Tigers Squad'])
      .mockReturnValueOnce(['Crimson Hawks Team']);
    renderComponent();

    await user.click(screen.getByRole('button', {name: 'Try another suggestion'}));
    await user.click(screen.getByText('Crimson Hawks Team'));

    expect(mockOnNameChange).toHaveBeenCalledWith('Crimson Hawks Team');
    expect(mockOnNameChange).toHaveBeenCalledTimes(1);
  });

  it('should update input value when name prop changes', () => {
    const {rerender} = renderComponent({name: 'Initial Name'});

    let input = screen.getByRole('textbox');
    expect(input).toHaveValue('Initial Name');

    rerender(<ConfigureName name="Updated Name" onNameChange={mockOnNameChange} />);

    input = screen.getByRole('textbox');
    expect(input).toHaveValue('Updated Name');
  });

  describe('length validation', () => {
    it('should show a maximum length error when the name is too long', () => {
      renderComponent({name: 'a'.repeat(101)});

      expect(screen.getByText('Group name cannot exceed 100 characters')).toBeInTheDocument();
    });

    it('should not show a length error when the name is empty', () => {
      renderComponent({name: ''});

      expect(screen.queryByText('Group name cannot exceed 100 characters')).not.toBeInTheDocument();
    });

    it('should not show a length error for a single character name', () => {
      renderComponent({name: 'A'});

      expect(screen.queryByText('Group name cannot exceed 100 characters')).not.toBeInTheDocument();
    });

    it('should not show a length error when the name is within bounds', () => {
      renderComponent({name: 'My Group'});

      expect(screen.queryByText('Group name cannot exceed 100 characters')).not.toBeInTheDocument();
    });
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with true when name is not empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({name: 'My Group', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when name is empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({name: '', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with false when name contains only whitespace', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({name: '   ', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with true for a single character name', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({name: 'A', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when name is longer than the maximum length', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({name: 'a'.repeat(101), onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should not crash when onReadyChange is undefined', () => {
      expect(() => {
        renderComponent({name: 'Test Group', onReadyChange: undefined});
      }).not.toThrow();
    });

    it('should call onReadyChange when name transitions from empty to non-empty', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = render(
        <ConfigureName name="" onNameChange={mockOnNameChange} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
      mockOnReadyChange.mockClear();

      rerender(<ConfigureName name="New Group" onNameChange={mockOnNameChange} onReadyChange={mockOnReadyChange} />);

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange when name transitions from non-empty to empty', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = render(
        <ConfigureName name="My Group" onNameChange={mockOnNameChange} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
      mockOnReadyChange.mockClear();

      rerender(<ConfigureName name="" onNameChange={mockOnNameChange} onReadyChange={mockOnReadyChange} />);

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });
  });
});
