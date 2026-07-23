/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ConfigureName, {type ConfigureNameProps} from '../ConfigureName';

// Mock the utility library
vi.mock('@thunderid/utils');

const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

describe('ConfigureName', () => {
  const mockOnAppNameChange = vi.fn();
  const mockSuggestions = ['My Web App', 'Customer Portal', 'Mobile App', 'Internal Dashboard'];

  const defaultProps: ConfigureNameProps = {
    appName: '',
    onAppNameChange: mockOnAppNameChange,
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

  it('should render the text field with correct label', () => {
    renderComponent();

    expect(screen.getByText('Application Name')).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toBeInTheDocument();
  });

  it('should display the current app name value', () => {
    renderComponent({appName: 'My Test App'});

    const input = screen.getByRole('textbox');
    expect(input).toHaveValue('My Test App');
  });

  it('should call onAppNameChange when typing in the input', async () => {
    const user = userEvent.setup();
    renderComponent();

    const input = screen.getByRole('textbox');
    await user.type(input, 'New App Name');

    expect(mockOnAppNameChange).toHaveBeenCalledTimes(12); // Once per character
    expect(mockOnAppNameChange).toHaveBeenLastCalledWith('e'); // Last character typed
  });

  it('should render name suggestions', () => {
    renderComponent();

    mockSuggestions.forEach((suggestion) => {
      expect(screen.getByText(suggestion)).toBeInTheDocument();
    });
  });

  it('should display suggestions label with icon', () => {
    renderComponent();

    expect(screen.getByText('In a hurry? Pick a random name:')).toBeInTheDocument();
  });

  it('should call onAppNameChange when clicking a suggestion chip', async () => {
    const user = userEvent.setup();
    renderComponent();

    const suggestionChip = screen.getByText('My Web App');
    await user.click(suggestionChip);

    expect(mockOnAppNameChange).toHaveBeenCalledWith('My Web App');
  });

  it('should render all suggestion chips as clickable', () => {
    renderComponent();

    mockSuggestions.forEach((suggestion) => {
      const chip = screen.getByText(suggestion);
      expect(chip.closest('div[role="button"]')).toBeInTheDocument();
    });
  });

  it('should generate suggestions only once on mount', () => {
    const {rerender} = renderComponent();

    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);

    rerender(<ConfigureName {...defaultProps} appName="Updated Name" />);

    // Should still be called only once due to useMemo
    expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);
  });

  it('should handle empty app name', () => {
    renderComponent({appName: ''});

    const input = screen.getByRole('textbox');
    expect(input).toHaveValue('');
  });

  it('should display placeholder text', () => {
    renderComponent();

    const input = screen.getByRole('textbox');
    expect(input).toHaveAttribute('placeholder');
  });

  it('should render required field indicator', () => {
    renderComponent();

    // FormControl with required prop should render asterisk or required indicator
    const label = screen.getByText('Application Name');
    expect(label).toBeInTheDocument();
    // Check for the asterisk in the label's parent (which should be a <label> element)
    const labelElement = label.closest('label');
    expect(labelElement).toHaveClass('Mui-required');
  });

  it('should handle special characters in app name', async () => {
    const user = userEvent.setup();
    renderComponent();

    const input = screen.getByRole('textbox');
    const specialName = 'App @#$ 123!';
    await user.type(input, specialName);

    // Each character is typed individually, so check that special characters triggered the callback
    expect(mockOnAppNameChange).toHaveBeenCalledWith('@');
    expect(mockOnAppNameChange).toHaveBeenCalledWith('#');
    expect(mockOnAppNameChange).toHaveBeenCalledWith('$');
    expect(mockOnAppNameChange).toHaveBeenCalledWith('!');
  });

  it('should update input value when appName prop changes', () => {
    const {rerender} = renderComponent({appName: 'Initial Name'});

    let input = screen.getByRole('textbox');
    expect(input).toHaveValue('Initial Name');

    rerender(<ConfigureName appName="Updated Name" onAppNameChange={mockOnAppNameChange} />);

    input = screen.getByRole('textbox');
    expect(input).toHaveValue('Updated Name');
  });

  it('should allow clearing the input', async () => {
    const user = userEvent.setup();
    renderComponent({appName: 'Some App'});

    const input = screen.getByRole('textbox');
    await user.clear(input);

    expect(mockOnAppNameChange).toHaveBeenCalledWith('');
  });

  it('should handle rapid suggestion clicks', async () => {
    const user = userEvent.setup();
    renderComponent();

    const firstSuggestion = screen.getByText('My Web App');
    const secondSuggestion = screen.getByText('Customer Portal');

    await user.click(firstSuggestion);
    await user.click(secondSuggestion);

    expect(mockOnAppNameChange).toHaveBeenCalledWith('My Web App');
    expect(mockOnAppNameChange).toHaveBeenCalledWith('Customer Portal');
    expect(mockOnAppNameChange).toHaveBeenCalledTimes(2);
  });

  it('should display lightbulb icon for suggestions', () => {
    renderComponent();

    // Check that the Lightbulb component is rendered (it's from lucide-react)
    const suggestionsSection = screen.getByText('In a hurry? Pick a random name:').closest('div');
    expect(suggestionsSection).toBeInTheDocument();
  });

  it('should handle long app names', async () => {
    const user = userEvent.setup();
    const longName = 'A'.repeat(100);
    renderComponent();

    const input = screen.getByRole('textbox');
    await user.type(input, longName);

    // Each character is typed individually
    expect(mockOnAppNameChange).toHaveBeenCalledTimes(100);
    expect(mockOnAppNameChange).toHaveBeenCalledWith('A');
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with true when appName is not empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({appName: 'My App', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when appName is empty', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({appName: '', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with false when appName contains only whitespace', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({appName: '   ', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should not crash when onReadyChange is undefined', () => {
      // This test ensures the component handles undefined onReadyChange gracefully
      expect(() => {
        renderComponent({appName: 'Test App', onReadyChange: undefined});
      }).not.toThrow();
    });

    it('should call onReadyChange when appName transitions from empty to non-empty', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = render(
        <ConfigureName appName="" onAppNameChange={mockOnAppNameChange} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
      mockOnReadyChange.mockClear();

      rerender(
        <ConfigureName appName="New App" onAppNameChange={mockOnAppNameChange} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange when appName transitions from non-empty to empty', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = render(
        <ConfigureName appName="My App" onAppNameChange={mockOnAppNameChange} onReadyChange={mockOnReadyChange} />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
      mockOnReadyChange.mockClear();

      rerender(<ConfigureName appName="" onAppNameChange={mockOnAppNameChange} onReadyChange={mockOnReadyChange} />);

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });
  });

  describe('duplicate name detection', () => {
    const duplicateMessage = 'An application with this name already exists. Choose a different name.';

    it('should show an inline error and block readiness for an exact duplicate name', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({appName: 'My App', existingAppNames: ['My App'], onReadyChange: mockOnReadyChange});

      expect(screen.getByText(duplicateMessage)).toBeInTheDocument();
      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should not flag case-variant names as duplicates', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({appName: 'my app', existingAppNames: ['My App'], onReadyChange: mockOnReadyChange});

      expect(screen.queryByText(duplicateMessage)).not.toBeInTheDocument();
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should become ready again when the name is edited to a unique one', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = render(
        <ConfigureName
          appName="My App"
          existingAppNames={['My App']}
          onAppNameChange={mockOnAppNameChange}
          onReadyChange={mockOnReadyChange}
        />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
      mockOnReadyChange.mockClear();

      rerender(
        <ConfigureName
          appName="My App 2"
          existingAppNames={['My App']}
          onAppNameChange={mockOnAppNameChange}
          onReadyChange={mockOnReadyChange}
        />,
      );

      expect(screen.queryByText(duplicateMessage)).not.toBeInTheDocument();
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should behave as before when existingAppNames is omitted', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({appName: 'My App', onReadyChange: mockOnReadyChange});

      expect(screen.queryByText(duplicateMessage)).not.toBeInTheDocument();
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });
  });
});
