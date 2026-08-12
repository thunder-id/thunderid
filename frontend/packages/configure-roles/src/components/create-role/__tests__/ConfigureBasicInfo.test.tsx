// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import ConfigureBasicInfo from '../ConfigureBasicInfo';
import type {ConfigureBasicInfoProps} from '../ConfigureBasicInfo';

vi.mock('@thunderid/utils');

const mockSuggestions = ['Alpha Manager', 'Beta Editor', 'Gamma Viewer'];
const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

describe('ConfigureBasicInfo', () => {
  const mockOnNameChange = vi.fn();

  const defaultProps: ConfigureBasicInfoProps = {
    name: '',
    onNameChange: mockOnNameChange,
  };

  const renderComponent = (props = defaultProps) => render(<ConfigureBasicInfo {...props} />);

  beforeEach(() => {
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue(mockSuggestions);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render the component with title', () => {
    renderComponent();

    expect(screen.getByRole('heading')).toBeInTheDocument();
  });

  it('should render name text field', () => {
    renderComponent();

    expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
  });

  it('should display current name value', () => {
    renderComponent({...defaultProps, name: 'Test Role'});

    const nameInput = screen.getByLabelText(/name/i);
    expect(nameInput).toHaveValue('Test Role');
  });

  it('should call onNameChange when typing in name input', async () => {
    const user = userEvent.setup();
    renderComponent();

    const nameInput = screen.getByLabelText(/name/i);
    await user.type(nameInput, 'A');

    expect(mockOnNameChange).toHaveBeenCalledWith('A');
  });

  it('should render a name suggestion', () => {
    renderComponent();

    expect(screen.getByText('Alpha Manager')).toBeInTheDocument();
  });

  it('should call onNameChange when clicking a suggestion chip', async () => {
    const user = userEvent.setup();
    renderComponent();

    const suggestionChip = screen.getByText('Alpha Manager');
    await user.click(suggestionChip);

    expect(mockOnNameChange).toHaveBeenCalledWith('Alpha Manager');
  });

  it('should generate suggestions only once on mount', () => {
    const {rerender} = renderComponent();

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);

    rerender(<ConfigureBasicInfo {...defaultProps} name="Updated" />);

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);
  });

  it('should handle special characters in name', async () => {
    const user = userEvent.setup();
    renderComponent();

    const nameInput = screen.getByLabelText(/name/i);
    await user.type(nameInput, '@');

    expect(mockOnNameChange).toHaveBeenCalledWith('@');
  });

  it('should update input values when props change', () => {
    const {rerender} = renderComponent({...defaultProps, name: 'Initial Name'});

    let nameInput = screen.getByLabelText(/name/i);
    expect(nameInput).toHaveValue('Initial Name');

    rerender(<ConfigureBasicInfo {...defaultProps} name="Updated Name" />);

    nameInput = screen.getByLabelText(/name/i);
    expect(nameInput).toHaveValue('Updated Name');
  });

  describe('length validation', () => {
    it('should show a maximum length error when the name is too long', () => {
      renderComponent({...defaultProps, name: 'a'.repeat(101)});

      expect(screen.getByText('Role name cannot exceed 100 characters')).toBeInTheDocument();
    });

    it('should not show a length error when the name is empty', () => {
      renderComponent({...defaultProps, name: ''});

      expect(screen.queryByText('Role name cannot exceed 100 characters')).not.toBeInTheDocument();
    });

    it('should not show a length error for a single character name', () => {
      renderComponent({...defaultProps, name: 'A'});

      expect(screen.queryByText('Role name cannot exceed 100 characters')).not.toBeInTheDocument();
    });

    it('should not show a length error when the name is within bounds', () => {
      renderComponent({...defaultProps, name: 'My Role'});

      expect(screen.queryByText('Role name cannot exceed 100 characters')).not.toBeInTheDocument();
    });
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with true when name is not empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: 'My Role', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when name is empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: '', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with false when name contains only whitespace', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: '   ', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with true for a single character name', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: 'A', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when name is longer than the maximum length', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({...defaultProps, name: 'a'.repeat(101), onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should not crash when onReadyChange is undefined', () => {
      expect(() => {
        renderComponent({...defaultProps, onReadyChange: undefined});
      }).not.toThrow();
    });
  });
});
