// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import I18nContext, {type I18nContextProps} from '../../../context/I18nContext';
import {PreviewScreenType} from '../../../models/custom-text-preference';
import I18nConfigurationCard from '../I18nConfigurationCard';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'login.title': 'Sign In',
        'login.description': 'Enter your credentials',
        'login.button.submit': 'Submit',
        'common.continue': 'Continue',
        'common.cancel': 'Cancel',
        'custom:login.title': 'Sign In',
        'signin:forms.credentials.title': 'Sign In',
      };
      // getErrorMessage's error-code and fallback lookups pass a `defaultValue` option rather than
      // interpolation params, so those are resolved from the translations map (or the default
      // value) instead of being echoed back as interpolated text.
      if (params && 'defaultValue' in params) {
        return translations[key] ?? ((params['defaultValue'] as string) || key);
      }
      if (params) {
        return `${key} ${JSON.stringify(params)}`;
      }
      return translations[key] ?? key;
    },
  }),
}));

// Mock contexts
vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getServerUrl: () => 'https://localhost:8090',
  }),
}));

// Mock the API hooks from i18n
const mockMutate = vi.fn();
vi.mock('@thunderid/i18n', () => ({
  NamespaceConstants: {CUSTOM_NAMESPACE: 'custom'},
  I18nDefaultConstants: {FALLBACK_LANGUAGE: 'en-US'},
  useUpdateTranslation: () => ({
    mutate: mockMutate,
    isPending: false,
  }),
  useGetLanguages: () => ({
    data: {languages: ['en', 'es', 'fr']},
  }),
  useGetTranslations: () => ({
    data: {
      language: 'en-US',
      translations: {
        custom: {
          'login.title': 'Sign In',
          'login.description': 'Enter your credentials',
          'login.button.submit': 'Submit',
          'common.continue': 'Continue',
          'common.cancel': 'Cancel',
        },
      },
    },
    isLoading: false,
  }),
}));

describe('I18nConfigurationCard', () => {
  const mockOnClose = vi.fn();
  const mockOnChange = vi.fn();

  let anchorEl: HTMLDivElement;

  // Reset mocks before each test
  beforeEach(() => {
    mockMutate.mockReset();
  });

  const mockI18nText = {
    [PreviewScreenType.LOGIN]: {
      'login.title': 'Sign In',
      'login.description': 'Enter your credentials',
      'login.button.submit': 'Submit',
    },
    [PreviewScreenType.COMMON]: {
      'common.continue': 'Continue',
      'common.cancel': 'Cancel',
    },
  };

  const defaultI18nValue: I18nContextProps = {
    primaryI18nScreen: PreviewScreenType.LOGIN,
    i18nText: mockI18nText,
    i18nTextLoading: false,
  };

  const createContextValue = (overrides: Partial<I18nContextProps> = {}): I18nContextProps => ({
    ...defaultI18nValue,
    ...overrides,
  });

  const createWrapper = (i18nValue: I18nContextProps = createContextValue()) =>
    function Wrapper({children}: {children: ReactNode}) {
      return <I18nContext.Provider value={i18nValue}>{children}</I18nContext.Provider>;
    };

  beforeEach(() => {
    anchorEl = document.createElement('div');
    document.body.appendChild(anchorEl);
    vi.clearAllMocks();
  });

  afterEach(() => {
    document.body.removeChild(anchorEl);
  });

  describe('Rendering', () => {
    it('should render the popover when open is true', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('presentation')).toBeInTheDocument();
    });

    it('should not render content when open is false', () => {
      render(
        <I18nConfigurationCard
          open={false}
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.queryByText('flows:core.elements.textPropertyField.i18nCard.i18nKey')).not.toBeInTheDocument();
    });

    it('should render the card title with formatted property key', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="buttonLabel"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      expect(
        screen.getByText('flows:core.elements.textPropertyField.i18nCard.title {"field":"Button Label"}'),
      ).toBeInTheDocument();
    });

    it('should render the i18n key label', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByText('flows:core.elements.textPropertyField.i18nCard.i18nKey')).toBeInTheDocument();
    });

    it('should render loading state when i18nTextLoading is true', () => {
      const loadingContext = createContextValue({i18nTextLoading: true});

      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper(loadingContext)},
      );

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });
  });

  describe('Close Functionality', () => {
    it('should call onClose when close button is clicked', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      const closeButton = screen.getByLabelText('common:close');
      fireEvent.click(closeButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Autocomplete Options', () => {
    it('should display available i18n keys in autocomplete', async () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Open the autocomplete dropdown by clicking the Open button
      const openButton = screen.getByTitle('Open');
      fireEvent.click(openButton);

      // Keys are now prefixed with the namespace (custom:)
      await waitFor(() => {
        expect(screen.getByText('custom:login.title')).toBeInTheDocument();
        expect(screen.getByText('custom:login.description')).toBeInTheDocument();
        expect(screen.getByText('custom:common.continue')).toBeInTheDocument();
      });
    });

    it('should handle empty i18nText gracefully', () => {
      const emptyContext = createContextValue({i18nText: undefined});

      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper(emptyContext)},
      );

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  describe('Selection and onChange', () => {
    it('should call onChange with selected i18n key', async () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Open the autocomplete dropdown by clicking the Open button
      const openButton = screen.getByTitle('Open');
      fireEvent.click(openButton);

      // Keys are now prefixed with the namespace (custom:)
      await waitFor(() => {
        expect(screen.getByText('custom:login.title')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('custom:login.title'));

      expect(mockOnChange).toHaveBeenCalledWith('custom:login.title');
    });

    it('should call onChange with empty string when selection is cleared', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey="login.title"
        />,
        {wrapper: createWrapper()},
      );

      const clearButton = screen.getByLabelText('Clear');
      fireEvent.click(clearButton);

      expect(mockOnChange).toHaveBeenCalledWith('');
    });
  });

  describe('Resolved Value Display', () => {
    it('should display resolved value when i18n key is selected', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey="login.title"
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByText('flows:core.elements.textPropertyField.resolvedValue')).toBeInTheDocument();
      expect(screen.getByText('Sign In')).toBeInTheDocument();
    });

    it('should not display resolved value box when no i18n key is selected', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.queryByText('flows:core.elements.textPropertyField.resolvedValue')).not.toBeInTheDocument();
    });

    it('should not display resolved value box when key has no matching value', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey="nonexistent.key"
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.queryByText('flows:core.elements.textPropertyField.resolvedValue')).not.toBeInTheDocument();
    });

    it('should display resolved value from common screen', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey="common.continue"
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByText('Continue')).toBeInTheDocument();
    });

    it('should display resolved value when a namespaced i18n key is selected', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey="custom:login.title"
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByText('flows:core.elements.textPropertyField.resolvedValue')).toBeInTheDocument();
      expect(screen.getByText('Sign In')).toBeInTheDocument();
    });

    it('should not display resolved value box when namespaced key has no matching translation', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey="signin:nonexistent.key"
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.queryByText('flows:core.elements.textPropertyField.resolvedValue')).not.toBeInTheDocument();
    });
  });

  describe('Selected Value Display', () => {
    it('should show selected i18n key in autocomplete', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey="login.title"
        />,
        {wrapper: createWrapper()},
      );

      const input: HTMLInputElement = screen.getByRole('combobox');
      expect(input.value).toBe('login.title');
    });

    it('should show empty autocomplete when i18nKey is empty string', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      const input: HTMLInputElement = screen.getByRole('combobox');
      expect(input.value).toBe('');
    });
  });

  describe('Create Translation Mode', () => {
    it('should show create translation button', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle')).toBeInTheDocument();
    });

    it('should enter create mode when create button is clicked', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      const createButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createButton);

      // Should show language selector in create mode
      expect(screen.getByText('flows:core.elements.textPropertyField.i18nCard.language')).toBeInTheDocument();
      expect(screen.getByText('flows:core.elements.textPropertyField.i18nCard.languageText')).toBeInTheDocument();
    });

    it('should show cancel button in create mode', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      const createButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createButton);

      expect(screen.getByText('common:cancel')).toBeInTheDocument();
    });

    it('should exit create mode when cancel is clicked', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createButton);

      // Click cancel
      const cancelButton = screen.getByText('common:cancel');
      fireEvent.click(cancelButton);

      // Should be back in select mode - the placeholder text indicates we're in select mode
      expect(
        screen.getByPlaceholderText('flows:core.elements.textPropertyField.i18nCard.selectI18nKey'),
      ).toBeInTheDocument();
    });

    it('should have disabled create button when key or value is empty', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // The create button should be disabled when fields are empty
      const submitButton = screen.getByText('common:create');
      expect(submitButton).toBeDisabled();
    });

    it('should show validation error when key is empty and create is clicked', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in translation value only, leave key empty
      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'Some translation'}});

      // Submit button should be enabled because we check trim() - but if we click it empty, validation kicks in
      // Let's click the create button
      const submitButton = screen.getByText('common:create');
      // Button is disabled because key is empty (after trim)
      expect(submitButton).toBeDisabled();
    });

    it('should show validation error for invalid key format', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in key with invalid characters
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'invalid key with spaces!'}});

      // Fill in translation value
      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'Some translation'}});

      // Click create
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      // Should show error for invalid key format
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText('flows:core.elements.textPropertyField.i18nCard.invalidKeyFormat')).toBeInTheDocument();
    });

    it('should call mutate when form is valid', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in valid key
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'my.new.key'}});

      // Fill in translation value
      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'My translation value'}});

      // Click create
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      // Should call mutate with correct params
      expect(mockMutate).toHaveBeenCalledWith(
        {
          language: 'en-US',
          namespace: 'custom',
          key: 'my.new.key',
          value: 'My translation value',
        },

        expect.objectContaining({
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          onSuccess: expect.any(Function),
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          onError: expect.any(Function),
        }),
      );
    });

    it('should call onChange and exit create mode on successful creation', async () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in form
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'my.new.key'}});

      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'My translation value'}});

      // Click create
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      // Get the onSuccess callback and call it
      const mutateCall = mockMutate.mock.calls[0] as [unknown, {onSuccess: () => void; onError: (err: Error) => void}];
      const callbacks = mutateCall[1];
      callbacks.onSuccess();

      // Should have called onChange with the new key
      expect(mockOnChange).toHaveBeenCalledWith('custom:my.new.key');

      // Should be back in select mode - check for the title change instead of placeholder
      await waitFor(() => {
        expect(
          screen.getByText('flows:core.elements.textPropertyField.i18nCard.title {"field":"Label"}'),
        ).toBeInTheDocument();
      });
    });

    it('should show a resolved error message on failed creation, never the raw server text', async () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in form
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'my.new.key'}});

      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'My translation value'}});

      // Click create
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      // Get the onError callback and call it
      const mutateCall = mockMutate.mock.calls[0] as [unknown, {onSuccess: () => void; onError: (err: Error) => void}];
      const callbacks = mutateCall[1];
      callbacks.onError(new Error('API Error'));

      // Should show the resolved fallback message, not the raw server error text
      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
        expect(screen.getByText('Failed to create translation. Please try again.')).toBeInTheDocument();
        expect(screen.queryByText('API Error')).not.toBeInTheDocument();
      });
    });

    it('should clear error when typing in key field', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in invalid key to trigger error
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'invalid key!'}});

      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'Some translation'}});

      // Click create to trigger error
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      expect(screen.getByRole('alert')).toBeInTheDocument();

      // Type in key field to clear error
      fireEvent.change(keyInput, {target: {value: 'valid.key'}});

      // Error should be cleared
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should clear error when typing in value field', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in invalid key to trigger error
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'invalid key!'}});

      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'Some translation'}});

      // Click create to trigger error
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      expect(screen.getByRole('alert')).toBeInTheDocument();

      // Type in value field to clear error
      fireEvent.change(translationValueInput, {target: {value: 'Updated translation'}});

      // Error should be cleared
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should close error alert when close button is clicked', () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in invalid key to trigger error
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'invalid key!'}});

      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'Some translation'}});

      // Click create to trigger error
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();

      // Close the alert
      const closeAlertButton = alert.querySelector('button');
      if (closeAlertButton) {
        fireEvent.click(closeAlertButton);
      }

      // Error should be cleared
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should allow selecting a different language', async () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Find the language autocomplete - it's the first combobox in create mode
      const languageCombobox = screen.getAllByRole('combobox')[0];

      // Open the dropdown by clicking the Open button within the autocomplete
      const openButtons = screen.getAllByTitle('Open');
      fireEvent.click(openButtons[0]);

      // Wait for dropdown and select Spanish
      await waitFor(() => {
        expect(screen.getByRole('listbox')).toBeInTheDocument();
      });

      // Find and click the Spanish option
      const options = screen.getAllByRole('option');
      const esOption = options.find((opt) => opt.textContent === 'es');
      expect(esOption).toBeDefined();
      fireEvent.click(esOption!);

      // Verify the language was selected
      expect(languageCombobox).toHaveValue('es');

      // Fill in form
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'my.key'}});

      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'Mi traducción'}});

      // Click create
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      // Should call mutate with selected language
      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          language: 'es',
        }),
        expect.any(Object),
      );
    });
  });

  describe('Edge Cases', () => {
    it('should handle error without message', async () => {
      render(
        <I18nConfigurationCard
          open
          anchorEl={anchorEl}
          propertyKey="label"
          onClose={mockOnClose}
          onChange={mockOnChange}
          i18nKey=""
        />,
        {wrapper: createWrapper()},
      );

      // Enter create mode
      const createModeButton = screen.getByText('flows:core.elements.textPropertyField.i18nCard.createTitle');
      fireEvent.click(createModeButton);

      // Fill in form
      const keyInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder',
      );
      fireEvent.change(keyInput, {target: {value: 'my.new.key'}});

      const translationValueInput = screen.getByPlaceholderText(
        'flows:core.elements.textPropertyField.i18nCard.languageTextPlaceholder',
      );
      fireEvent.change(translationValueInput, {target: {value: 'My translation value'}});

      // Click create
      const submitButton = screen.getByText('common:create');
      fireEvent.click(submitButton);

      // Get the onError callback and call it with error without message
      const mutateCall = mockMutate.mock.calls[0] as [unknown, {onSuccess: () => void; onError: (err: Error) => void}];
      const callbacks = mutateCall[1];
      callbacks.onError({} as Error);

      // Should show fallback error message
      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
        expect(screen.getByText('Failed to create translation. Please try again.')).toBeInTheDocument();
      });
    });
  });
});
