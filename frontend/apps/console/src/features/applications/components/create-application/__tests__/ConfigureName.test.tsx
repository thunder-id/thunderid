// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ConfigureName, {type ConfigureNameProps} from '../ConfigureName';

// Mock the utility library
vi.mock('@thunderid/utils');

// Mock the shared logo picker so tests only assert on the wiring, not LogoPicker's own behavior.
// NameSuggestion is left as the real implementation since these tests exercise its wiring too.
vi.mock('@thunderid/components', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/components')>();
  return {
    ...actual,
    ResourceAvatar: ({
      value,
      onSelect,
      editAriaLabel,
    }: {
      value: string;
      onSelect: (value: string) => void;
      editAriaLabel: string;
    }) => (
      <button
        type="button"
        data-testid="resource-avatar"
        aria-label={editAriaLabel}
        onClick={() => onSelect('emoji:🚀')}
      >
        {value}
      </button>
    ),
  };
});

vi.mock('@thunderid/react', () => ({
  buildAvatarSpec: vi.fn(() => 'avatar:shape=rounded,variant=anonymous_entity,content=briefcase,colors=0'),
  pickAnonymousEntityName: vi.fn(() => 'briefcase'),
}));

const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

describe('ConfigureName', () => {
  const mockOnAppNameChange = vi.fn();
  const mockOnLogoSelect = vi.fn();
  const mockSuggestion = 'Wise Clocks Run';

  const defaultProps: ConfigureNameProps = {
    appName: '',
    onAppNameChange: mockOnAppNameChange,
    appLogo: 'emoji:🐼',
    onLogoSelect: mockOnLogoSelect,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue([mockSuggestion]);
  });

  const renderComponent = (props: Partial<ConfigureNameProps> = {}) =>
    render(<ConfigureName {...defaultProps} {...props} />);

  it('should render the component with title', () => {
    renderComponent();

    expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
  });

  it('should not render the title when showTitle is false', () => {
    renderComponent({showTitle: false});

    expect(screen.queryByRole('heading', {level: 1})).not.toBeInTheDocument();
  });

  it('should render the text field with correct label', () => {
    renderComponent();

    expect(screen.getByText('Name & Logo')).toBeInTheDocument();
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

  it('should render required field indicator', () => {
    renderComponent();

    const label = screen.getByText('Name & Logo');
    expect(label).toBeInTheDocument();
    const labelElement = label.closest('label');
    expect(labelElement).toHaveClass('Mui-required');
  });

  it('should handle special characters in app name', async () => {
    const user = userEvent.setup();
    renderComponent();

    const input = screen.getByRole('textbox');
    const specialName = 'App @#$ 123!';
    await user.type(input, specialName);

    expect(mockOnAppNameChange).toHaveBeenCalledWith('@');
    expect(mockOnAppNameChange).toHaveBeenCalledWith('#');
    expect(mockOnAppNameChange).toHaveBeenCalledWith('$');
    expect(mockOnAppNameChange).toHaveBeenCalledWith('!');
  });

  it('should update input value when appName prop changes', () => {
    const {rerender} = renderComponent({appName: 'Initial Name'});

    let input = screen.getByRole('textbox');
    expect(input).toHaveValue('Initial Name');

    rerender(
      <ConfigureName
        appName="Updated Name"
        onAppNameChange={mockOnAppNameChange}
        appLogo="emoji:🐼"
        onLogoSelect={mockOnLogoSelect}
      />,
    );

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

  it('should handle long app names', async () => {
    const user = userEvent.setup();
    const longName = 'A'.repeat(100);
    renderComponent();

    const input = screen.getByRole('textbox');
    await user.type(input, longName);

    expect(mockOnAppNameChange).toHaveBeenCalledTimes(100);
    expect(mockOnAppNameChange).toHaveBeenCalledWith('A');
  });

  describe('logo picker', () => {
    it('should render the logo picker inline with the name field', () => {
      renderComponent({appLogo: 'emoji:🐼'});

      expect(screen.getByTestId('resource-avatar')).toHaveTextContent('emoji:🐼');
    });

    it('should call onLogoSelect when a new logo is picked', async () => {
      const user = userEvent.setup();
      renderComponent({appLogo: 'emoji:🐼'});

      await user.click(screen.getByTestId('resource-avatar'));

      expect(mockOnLogoSelect).toHaveBeenCalledWith('emoji:🚀');
    });

    it('should auto-select a default entity avatar when appLogo is empty', () => {
      renderComponent({appLogo: null});

      expect(mockOnLogoSelect).toHaveBeenCalledWith(
        'avatar:shape=rounded,variant=anonymous_entity,content=briefcase,colors=0',
      );
    });

    it('should not auto-select when appLogo is already set', () => {
      renderComponent({appLogo: 'emoji:🐼'});

      expect(mockOnLogoSelect).not.toHaveBeenCalled();
    });
  });

  describe('name suggestion', () => {
    it('should render a single suggestion instead of a list', () => {
      renderComponent();

      expect(screen.getByText(mockSuggestion)).toBeInTheDocument();
    });

    it('should generate the suggestion only once on mount', () => {
      const {rerender} = renderComponent();

      expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);

      rerender(
        <ConfigureName
          appName="Updated Name"
          onAppNameChange={mockOnAppNameChange}
          appLogo="emoji:🐼"
          onLogoSelect={mockOnLogoSelect}
        />,
      );

      expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(1);
    });

    it('should call onAppNameChange when clicking the suggestion', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByText(mockSuggestion));

      expect(mockOnAppNameChange).toHaveBeenCalledWith(mockSuggestion);
    });

    it('should request a new suggestion when the shuffle button is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(generateRandomHumanReadableIdentifiers)
        .mockReturnValueOnce([mockSuggestion])
        .mockReturnValueOnce(['Fine Cobras Pay']);
      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Try another suggestion'}));

      expect(generateRandomHumanReadableIdentifiers).toHaveBeenCalledTimes(2);
      expect(screen.getByText('Fine Cobras Pay')).toBeInTheDocument();
    });
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
      expect(() => {
        renderComponent({appName: 'Test App', onReadyChange: undefined});
      }).not.toThrow();
    });

    it('should call onReadyChange when appName transitions from empty to non-empty', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = renderComponent({appName: '', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
      mockOnReadyChange.mockClear();

      rerender(
        <ConfigureName
          appName="New App"
          onAppNameChange={mockOnAppNameChange}
          appLogo="emoji:🐼"
          onLogoSelect={mockOnLogoSelect}
          onReadyChange={mockOnReadyChange}
        />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange when appName transitions from non-empty to empty', () => {
      const mockOnReadyChange = vi.fn();
      const {rerender} = renderComponent({appName: 'My App', onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
      mockOnReadyChange.mockClear();

      rerender(
        <ConfigureName
          appName=""
          onAppNameChange={mockOnAppNameChange}
          appLogo="emoji:🐼"
          onLogoSelect={mockOnLogoSelect}
          onReadyChange={mockOnReadyChange}
        />,
      );

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
          appLogo={defaultProps.appLogo}
          onLogoSelect={mockOnLogoSelect}
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
          appLogo={defaultProps.appLogo}
          onLogoSelect={mockOnLogoSelect}
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
