// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ValidationContext, type ValidationContextProps} from '../../../context/ValidationContext';
import Notification from '../../../models/notification';
import type {Resource} from '../../../models/resources';
import TextPropertyField from '../TextPropertyField';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, options?: {propertyName?: string}) =>
      options?.propertyName ? `Enter ${options.propertyName}` : key,
  }),
}));

// Mock contexts
vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getServerUrl: () => 'https://localhost:8090',
  }),
}));

// Mock the API hooks used by I18nConfigurationCard from i18n
vi.mock('@thunderid/i18n', () => ({
  NamespaceConstants: {CUSTOM_NAMESPACE: 'custom'},
  I18nDefaultConstants: {FALLBACK_LANGUAGE: 'en-US'},
  useUpdateTranslation: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
  useGetLanguages: () => ({
    data: {languages: ['en-US', 'es', 'fr']},
  }),
  useGetTranslations: () => ({
    data: {
      language: 'en-US',
      translations: {
        custom: {
          'common.submit': 'Submit',
          'common.button': 'Button',
          'common.label': 'Label',
          'common.test': 'Test',
          'login.submit': 'Log In',
        },
      },
    },
    isLoading: false,
  }),
}));

// Mock useI18nConfig used by I18nConfigurationCard
vi.mock('../../../hooks/useI18nConfig', () => ({
  default: () => ({
    i18nText: {},
    i18nTextLoading: false,
  }),
}));

describe('TextPropertyField', () => {
  const mockOnChange = vi.fn();

  const mockResource: Resource = {
    id: 'resource-1',
    type: 'TEXT',
    config: {},
  } as Resource;

  const defaultContextValue: ValidationContextProps = {
    isValid: true,
    notifications: [],
    getNotification: vi.fn(),
    validationConfig: {
      isOTPValidationEnabled: false,
      isRecoveryFactorValidationEnabled: false,
      isPasswordExecutorValidationEnabled: false,
    },
  };

  const createWrapper = (contextValue: ValidationContextProps = defaultContextValue) => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ValidationContext.Provider value={contextValue}>{children}</ValidationContext.Provider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render text field with label', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="userName"
          propertyValue="test"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // Check the label text is rendered
      expect(screen.getByText('User Name')).toBeInTheDocument();
      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    it('should convert camelCase propertyKey to Start Case label', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="myPropertyName"
          propertyValue=""
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // Check the label text is rendered in Start Case
      expect(screen.getByText('My Property Name')).toBeInTheDocument();
    });

    it('should render text field with default value', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="title"
          propertyValue="Hello World"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toHaveValue('Hello World');
    });

    it('should render empty text field when value is empty', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="title" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toHaveValue('');
    });
  });

  describe('onChange Handler', () => {
    it('should call onChange when text is entered', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="label" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      fireEvent.change(textField, {target: {value: 'New Value'}});

      expect(mockOnChange).toHaveBeenCalledWith('label', 'New Value', mockResource, true);
    });

    it('should pass the correct resource to onChange', () => {
      const specificResource = {...mockResource, id: 'specific-resource'};
      render(
        <TextPropertyField
          resource={specificResource}
          propertyKey="description"
          propertyValue=""
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      fireEvent.change(textField, {target: {value: 'Test'}});

      expect(mockOnChange).toHaveBeenCalledWith('description', 'Test', specificResource, true);
    });
  });

  describe('Error State', () => {
    it('should display error message when notification exists', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      notification.addResourceFieldNotification('resource-1_title', 'Title is required');

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <TextPropertyField resource={mockResource} propertyKey="title" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.getByText('Title is required')).toBeInTheDocument();
    });

    it('should not display error message when no notification exists', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="title" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should not display error message for different property', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      notification.addResourceFieldNotification('resource-1_otherProperty', 'Other error');

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <TextPropertyField resource={mockResource} propertyKey="title" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.queryByText('Other error')).not.toBeInTheDocument();
    });
  });

  describe('I18n Pattern', () => {
    it('should display resolved value box when value matches i18n pattern', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="{{t(common.submit)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // The text field should have the i18n key
      expect(screen.getByRole('textbox')).toHaveValue('{{t(common.submit)}}');
      // The resolved value is shown as a single caption line, labelled by tooltip.
      expect(screen.getByText('common.submit')).toBeInTheDocument();
    });

    it('should not display resolved value box for regular text', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="Regular Text"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // Should not show a resolved value preview for a plain literal.
      expect(screen.queryByLabelText('flows:core.elements.textPropertyField.resolvedValue')).not.toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('should have accessible text input', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="username" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toBeInTheDocument();
    });

    it('should render label element with htmlFor attribute', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="email" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      // Check that the label is rendered with the correct text
      const label = screen.getByText('Email');
      expect(label).toBeInTheDocument();
      // Check that the label has a for attribute (htmlFor in React)
      expect(label).toHaveAttribute('for');
    });
  });

  describe('TextField Props', () => {
    it('should keep i18n key in text field when i18n pattern is detected', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="{{t(common.button)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      // When i18n pattern is detected, value is kept in the text field
      expect(textField).toHaveValue('{{t(common.button)}}');
    });

    it('should render placeholder when not i18n pattern', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="title" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      // The placeholder should be set based on the translation
      expect(textField).toHaveAttribute('placeholder', 'Enter Title');
    });

    it('should render placeholder even when i18n pattern is detected', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="{{t(common.label)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      // The placeholder should still be shown for i18n patterns
      expect(textField).toHaveAttribute('placeholder', 'Enter Label');
    });
  });

  describe('Additional Props', () => {
    it('should pass additional props to TextField', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="username"
          propertyValue=""
          onChange={mockOnChange}
          disabled
        />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toBeDisabled();
    });

    it('should handle multiline prop', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="description"
          propertyValue=""
          onChange={mockOnChange}
          multiline
          rows={4}
        />,
        {wrapper: createWrapper()},
      );

      // Check if textarea is rendered (multiline makes TextField render as textarea)
      const textArea = screen.getByRole('textbox');
      expect(textArea).toBeInTheDocument();
    });
  });

  describe('I18n Key Extraction', () => {
    it('should extract i18n key from pattern with simple key', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="buttonLabel"
          propertyValue="{{t(login.submit)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // The i18n key should be kept in the input field
      expect(screen.getByRole('textbox')).toHaveValue('{{t(login.submit)}}');
      // The resolved value should be displayed below
      expect(screen.getByText('login.submit')).toBeInTheDocument();
    });

    it('should extract i18n key from pattern with nested key', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="message"
          propertyValue="{{t(flows.login.welcome.message)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // The i18n key should be kept in the input field
      expect(screen.getByRole('textbox')).toHaveValue('{{t(flows.login.welcome.message)}}');
      // The resolved value should be displayed below
      expect(screen.getByText('flows.login.welcome.message')).toBeInTheDocument();
    });
  });

  describe('Error State with TextField', () => {
    it('should set error prop on TextField when error message exists', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      notification.addResourceFieldNotification('resource-1_name', 'Name is required');

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <TextPropertyField resource={mockResource} propertyKey="name" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper(contextWithError)},
      );

      // Check that error styling is applied
      const textField = screen.getByRole('textbox');
      expect(textField).toBeInTheDocument();
      expect(screen.getByText('Name is required')).toBeInTheDocument();
    });
  });

  describe('Resource ID Edge Cases', () => {
    it('should handle resource with undefined id', () => {
      const resourceWithUndefinedId = {...mockResource, id: undefined} as unknown as Resource;

      render(
        <TextPropertyField
          resource={resourceWithUndefinedId}
          propertyKey="label"
          propertyValue="test"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    it('should handle resource with empty id', () => {
      const resourceWithEmptyId = {...mockResource, id: ''};

      render(
        <TextPropertyField
          resource={resourceWithEmptyId}
          propertyKey="label"
          propertyValue="test"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });
  });

  describe('Notification Edge Cases', () => {
    it('should return empty string when selectedNotification is undefined', () => {
      const contextWithNoNotification: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: undefined,
      };

      render(
        <TextPropertyField resource={mockResource} propertyKey="title" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper(contextWithNoNotification)},
      );

      // Should not display any error message
      const formHelperTexts = document.querySelectorAll('.MuiFormHelperText-root');
      expect(formHelperTexts.length).toBe(0);
    });

    it('should handle notification without matching field notification', () => {
      const notification = new Notification('notification-1', 'Error', 'error');
      // No resource field notification added

      const contextWithNotification: ValidationContextProps = {
        ...defaultContextValue,
        selectedNotification: notification,
      };

      render(
        <TextPropertyField resource={mockResource} propertyKey="title" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper(contextWithNotification)},
      );

      // Should not display error message
      expect(screen.queryByText('Title is required')).not.toBeInTheDocument();
    });
  });

  describe('I18n Configuration Card', () => {
    it('should render component with i18n pattern value', () => {
      // The I18nConfigurationCard is conditionally rendered based on isI18nCardOpen state
      // which is currently not toggleable (toggle is commented out in the component)
      // This test verifies the component still works correctly with i18n patterns

      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="{{t(common.test)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // Verify the component renders with i18n pattern
      expect(screen.getByRole('textbox')).toHaveValue('{{t(common.test)}}');
      expect(screen.getByText('common.test')).toBeInTheDocument();
    });

    it('should not render i18n resolved value box when pattern has empty key', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="{{t()}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // When the pattern {{t()}} has no key inside, resolved value should be empty
      // so the resolved value box should not render
      expect(screen.getByRole('textbox')).toHaveValue('{{t()}}');
      // The resolved value box should not be displayed since there's no resolved value
      expect(screen.queryByText('flows:core.elements.textPropertyField.resolvedValue')).not.toBeInTheDocument();
    });

    it('should not render i18n card when isI18nCardOpen is false (default)', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="{{t(common.test)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // The I18nConfigurationCard should not be rendered as isI18nCardOpen is false by default
      // and there's no UI element to toggle it (toggle is commented out)
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('should open i18n card when language button is clicked', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="label" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      // Find and click the language icon button
      const languageButton = screen.getByRole('button');
      fireEvent.click(languageButton);

      // The I18nConfigurationCard should now be open
      expect(screen.getByRole('presentation')).toBeInTheDocument();
    });

    it('should close i18n card when close button is clicked', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="label" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      // Open the card
      const languageButton = screen.getByRole('button');
      fireEvent.click(languageButton);

      // Verify card is open
      expect(screen.getByRole('presentation')).toBeInTheDocument();

      // Find and click the close button in the card
      const closeButton = screen.getByLabelText('common:close');
      fireEvent.click(closeButton);

      // The card should be closed
      expect(screen.queryByRole('presentation')).not.toBeInTheDocument();
    });

    it('should toggle i18n card open and closed', () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="label" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      // Get the language button
      const languageButtons = screen.getAllByRole('button');
      const languageButton = languageButtons[0];

      // Open the card
      fireEvent.click(languageButton);
      expect(screen.getByRole('presentation')).toBeInTheDocument();

      // Click again to toggle close (via close button since popover blocks the toggle button)
      const closeButton = screen.getByLabelText('common:close');
      fireEvent.click(closeButton);
      expect(screen.queryByRole('presentation')).not.toBeInTheDocument();
    });

    it('should call onChange with formatted i18n value when i18n key is selected', async () => {
      render(
        <TextPropertyField resource={mockResource} propertyKey="label" propertyValue="" onChange={mockOnChange} />,
        {wrapper: createWrapper()},
      );

      // Open the card
      const languageButton = screen.getByRole('button');
      fireEvent.click(languageButton);

      // Open the autocomplete dropdown
      const openButton = screen.getByTitle('Open');
      fireEvent.click(openButton);

      // Wait for options and select one
      await waitFor(() => {
        expect(screen.getByText('custom:common.submit')).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText('custom:common.submit'));

      // Verify onChange was called with the formatted i18n pattern
      expect(mockOnChange).toHaveBeenCalledWith('label', '{{t(custom:common.submit)}}', mockResource);
    });

    it('should call onChange with empty string when i18n key is cleared', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="label"
          propertyValue="{{t(common.test)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // Open the card
      const languageButton = screen.getAllByRole('button')[0];
      fireEvent.click(languageButton);

      // Clear the selection
      const clearButton = screen.getByLabelText('Clear');
      fireEvent.click(clearButton);

      // Verify onChange was called with empty string
      expect(mockOnChange).toHaveBeenCalledWith('label', '', mockResource);
    });
  });

  describe('I18n Pattern Edge Cases', () => {
    it('should handle i18n pattern with special characters in key', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="message"
          propertyValue="{{t(auth.login.error_message)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('textbox')).toHaveValue('{{t(auth.login.error_message)}}');
      expect(screen.getByText('auth.login.error_message')).toBeInTheDocument();
    });

    it('should handle i18n pattern with deeply nested key', () => {
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="title"
          propertyValue="{{t(app.module.feature.component.label)}}"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('textbox')).toHaveValue('{{t(app.module.feature.component.label)}}');
      expect(screen.getByText('app.module.feature.component.label')).toBeInTheDocument();
    });
  });

  describe('Suggestions', () => {
    const suggestions = ['row', 'column', 'row-reverse'];

    const renderWithSuggestions = (propertyValue = '') =>
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="direction"
          propertyValue={propertyValue}
          onChange={mockOnChange}
          suggestions={suggestions}
        />,
        {wrapper: createWrapper()},
      );

    it('should offer the suggestions in a dropdown', () => {
      renderWithSuggestions();

      fireEvent.keyDown(screen.getByRole('combobox'), {key: 'ArrowDown'});

      expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual(suggestions);
    });

    it('should commit a selected suggestion immediately', () => {
      renderWithSuggestions();

      fireEvent.keyDown(screen.getByRole('combobox'), {key: 'ArrowDown'});
      fireEvent.click(screen.getAllByRole('option').find((option) => option.textContent === 'column')!);

      expect(mockOnChange).toHaveBeenCalledWith('direction', 'column', mockResource);
    });

    it('should accept a custom value that is not suggested', () => {
      renderWithSuggestions();

      fireEvent.change(screen.getByRole('combobox'), {target: {value: 'safe center'}});

      expect(screen.getByRole('combobox')).toHaveValue('safe center');
      expect(mockOnChange).toHaveBeenCalledWith('direction', 'safe center', mockResource, true);
    });

    it('should keep the typed value while a debounced update is in flight', () => {
      // The parent only echoes the value back after a debounce, so the field has to
      // hold its own state or keystrokes get clobbered mid-typing.
      renderWithSuggestions();

      const input = screen.getByRole('combobox');
      fireEvent.change(input, {target: {value: 'col'}});
      fireEvent.change(input, {target: {value: 'colu'}});

      expect(input).toHaveValue('colu');
    });

    it('should hide the dynamic value button for structural properties', () => {
      // Layout values map straight to CSS, so i18n/meta templates do not apply.
      render(
        <TextPropertyField
          resource={mockResource}
          propertyKey="direction"
          propertyValue=""
          onChange={mockOnChange}
          suggestions={suggestions}
          supportsDynamicValue={false}
        />,
        {wrapper: createWrapper()},
      );

      // The i18n mock echoes the key back, so the tooltip label is the raw key.
      expect(
        screen.queryByLabelText('flows:core.elements.textPropertyField.tooltip.configureDynamicValue'),
      ).not.toBeInTheDocument();
    });

    it('should keep the dynamic value button when the property supports it', () => {
      renderWithSuggestions();

      expect(
        screen.getByLabelText('flows:core.elements.textPropertyField.tooltip.configureDynamicValue'),
      ).toBeInTheDocument();
    });

    it('should render the field label above the input', () => {
      renderWithSuggestions();

      expect(screen.getByText('Direction')).toBeInTheDocument();
    });
  });
});
