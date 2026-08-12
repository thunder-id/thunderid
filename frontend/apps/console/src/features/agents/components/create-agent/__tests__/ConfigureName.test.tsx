// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ConfigureName, {type ConfigureNameProps} from '../ConfigureName';

// Mock the utility library
vi.mock('@thunderid/utils');

const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

describe('ConfigureName', () => {
  const mockOnAgentNameChange = vi.fn();
  const mockSuggestions = ['Billing Service', 'Customer Sync', 'Inventory Bot', 'Analytics Worker'];

  const defaultProps: ConfigureNameProps = {
    agentName: '',
    onAgentNameChange: mockOnAgentNameChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue(mockSuggestions);
  });

  const renderComponent = (props: Partial<ConfigureNameProps> = {}) =>
    render(<ConfigureName {...defaultProps} {...props} />);

  it('should render the component with title', () => {
    renderComponent();

    expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
  });

  it('should render the text field with the correct label', () => {
    renderComponent();

    expect(screen.getByText('Agent name')).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toBeInTheDocument();
  });

  it('should display the current agent name value', () => {
    renderComponent({agentName: 'My Test Agent'});

    const input = screen.getByRole('textbox');
    expect(input).toHaveValue('My Test Agent');
  });

  it('should call onAgentNameChange when typing in the input', async () => {
    const user = userEvent.setup();
    renderComponent();

    const input = screen.getByRole('textbox');
    await user.type(input, 'Hello');

    expect(mockOnAgentNameChange).toHaveBeenCalledTimes(5);
    expect(mockOnAgentNameChange).toHaveBeenLastCalledWith('o');
  });

  it('should render a name suggestion', () => {
    renderComponent();

    expect(screen.getByText('Billing Service')).toBeInTheDocument();
  });

  it('should call onAgentNameChange when clicking the suggestion', async () => {
    const user = userEvent.setup();
    renderComponent();

    const suggestion = screen.getByText('Billing Service');
    await user.click(suggestion);

    expect(mockOnAgentNameChange).toHaveBeenCalledWith('Billing Service');
  });

  it('should generate suggestions only once on mount', () => {
    const {rerender} = renderComponent();

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);

    rerender(<ConfigureName {...defaultProps} agentName="Updated Name" />);

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);
  });

  it('should display placeholder text', () => {
    renderComponent();

    const input = screen.getByRole('textbox');
    expect(input).toHaveAttribute('placeholder');
  });

  it('should allow clearing the input', async () => {
    const user = userEvent.setup();
    renderComponent({agentName: 'Some Agent'});

    const input = screen.getByRole('textbox');
    await user.clear(input);

    expect(mockOnAgentNameChange).toHaveBeenCalledWith('');
  });

  describe('length validation', () => {
    it('should show a maximum length error when the name is too long', () => {
      renderComponent({agentName: 'a'.repeat(101)});

      expect(screen.getByText('Agent name cannot exceed 100 characters')).toBeInTheDocument();
    });

    it('should not show a length error when the name is empty', () => {
      renderComponent({agentName: ''});

      expect(screen.queryByText('Agent name cannot exceed 100 characters')).not.toBeInTheDocument();
    });

    it('should not show a length error for a single character name', () => {
      renderComponent({agentName: 'A'});

      expect(screen.queryByText('Agent name cannot exceed 100 characters')).not.toBeInTheDocument();
    });

    it('should not show a length error when the name is at the maximum length', () => {
      renderComponent({agentName: 'a'.repeat(100)});

      expect(screen.queryByText('Agent name cannot exceed 100 characters')).not.toBeInTheDocument();
    });
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with true when agentName is not empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({agentName: 'My Agent', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when agentName is empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({agentName: '', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with false when agentName contains only whitespace', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({agentName: '   ', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with true for a single character name', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({agentName: 'A', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when agentName exceeds the maximum length', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({agentName: 'a'.repeat(101), onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should not crash when onReadyChange is undefined', () => {
      expect(() => {
        renderComponent({agentName: 'Test Agent', onReadyChange: undefined});
      }).not.toThrow();
    });

    it('should call onReadyChange when agentName transitions from empty to non-empty', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = render(
        <ConfigureName agentName="" onAgentNameChange={mockOnAgentNameChange} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
      mockOnReadyChange.mockClear();

      rerender(
        <ConfigureName
          agentName="New Agent"
          onAgentNameChange={mockOnAgentNameChange}
          onReadyChange={mockOnReadyChange}
        />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });
  });
});
