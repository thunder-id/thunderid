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

import {render, screen, fireEvent} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowBuilderElementConstants from '../../../constants/FlowBuilderElementConstants';
import {ValidationContext, type ValidationContextProps} from '../../../context/ValidationContext';
import {ElementTypes} from '../../../models/elements';
import type {Resource} from '../../../models/resources';
import CommonElementPropertyFactory from '../CommonElementPropertyFactory';

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
    withComponent: vi.fn().mockReturnThis(),
  }),
}));

// Mock icons package so ICON_NAMES is predictable and icon rendering works
vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  const MockUserIcon = Object.assign(({size}: {size?: number}) => <svg data-testid="icon-user" data-size={size} />, {
    displayName: 'User',
    $$typeof: Symbol.for('react.forward_ref'),
  });
  return {
    ...actual,
    User: MockUserIcon,
  };
});

// Mock child components
vi.mock('../rich-text/RichTextWithTranslation', () => ({
  default: ({onChange}: {onChange: (html: string) => void}) => (
    <div data-testid="rich-text-with-translation">
      <button type="button" onClick={() => onChange('<p>test</p>')}>
        Rich Text Editor
      </button>
    </div>
  ),
}));

vi.mock('../rich-text/RichTextActionFields', () => ({
  default: () => <div data-testid="rich-text-action-fields" />,
}));

vi.mock('../CheckboxPropertyField', () => ({
  default: ({
    resource,
    propertyKey,
    propertyValue,
    onChange,
  }: {
    resource: Resource;
    propertyKey: string;
    propertyValue: boolean;
    onChange: (key: string, value: boolean, resource: Resource) => void;
  }) => (
    <div data-testid="checkbox-property-field">
      <input
        type="checkbox"
        checked={propertyValue}
        onChange={(e) => onChange(propertyKey, e.target.checked, resource)}
        data-property-key={propertyKey}
      />
    </div>
  ),
}));

vi.mock('../TextPropertyField', () => ({
  default: ({
    resource,
    propertyKey,
    propertyValue,
    onChange,
  }: {
    resource: Resource;
    propertyKey: string;
    propertyValue: string;
    onChange: (key: string, value: string, resource: Resource) => void;
  }) => (
    <div data-testid="text-property-field">
      <input
        type="text"
        value={propertyValue}
        onChange={(e) => onChange(propertyKey, e.target.value, resource)}
        data-property-key={propertyKey}
      />
    </div>
  ),
}));

describe('CommonElementPropertyFactory', () => {
  const mockOnChange = vi.fn();

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

  describe('RichText Element', () => {
    it('should render RichTextWithTranslation for label property when resource is RichText', () => {
      const richTextResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.RichText,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={richTextResource}
          propertyKey="label"
          propertyValue="<p>Test content</p>"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('rich-text-with-translation')).toBeInTheDocument();
    });

    it('should not render RichTextWithTranslation for non-label properties on RichText', () => {
      const richTextResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.RichText,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={richTextResource}
          propertyKey="other"
          propertyValue="test"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.queryByTestId('rich-text-with-translation')).not.toBeInTheDocument();
      expect(screen.getByTestId('text-property-field')).toBeInTheDocument();
    });
  });

  describe('Boolean Properties', () => {
    it('should render CheckboxPropertyField for boolean property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="required"
          propertyValue
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('checkbox-property-field')).toBeInTheDocument();
    });

    it('should render CheckboxPropertyField for false boolean values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="disabled"
          propertyValue={false}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('checkbox-property-field')).toBeInTheDocument();
    });
  });

  describe('String Properties', () => {
    it('should render TextPropertyField for string property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="placeholder"
          propertyValue="Enter text"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('text-property-field')).toBeInTheDocument();
    });

    it('should render TextPropertyField for empty string values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="hint"
          propertyValue=""
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('text-property-field')).toBeInTheDocument();
    });
  });

  describe('Captcha Element', () => {
    it('should render TextField with default provider for Captcha resource', () => {
      const captchaResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Captcha,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={captchaResource}
          propertyKey="provider"
          propertyValue={undefined}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toBeInTheDocument();
      expect(textField).toHaveValue(FlowBuilderElementConstants.DEFAULT_CAPTCHA_PROVIDER);
    });

    it('should render disabled TextField for Captcha provider', () => {
      const captchaResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Captcha,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={captchaResource}
          propertyKey="provider"
          propertyValue={null}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toBeDisabled();
    });
  });

  describe('Null Cases', () => {
    it('should return null for unsupported property types', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      const {container} = render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="complexProp"
          propertyValue={{nested: 'object'}}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(container.firstChild).toBeNull();
    });

    it('should render a text field for number property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      const {getByTestId} = render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="count"
          propertyValue={42}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(getByTestId('text-property-field')).toBeDefined();
    });

    it('should return null for array property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      const {container} = render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="items"
          propertyValue={['item1', 'item2']}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Additional Props', () => {
    it('should pass additional props to child components', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="label"
          propertyValue="Test Label"
          onChange={mockOnChange}
          data-custom-prop="custom-value"
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('text-property-field')).toBeInTheDocument();
    });
  });

  describe('Icon Element', () => {
    it('should render Autocomplete for Icon element with name property', () => {
      const iconResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Icon,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={iconResource}
          propertyKey="name"
          propertyValue="User"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should render Autocomplete for Icon element with empty propertyValue', () => {
      const iconResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Icon,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={iconResource}
          propertyKey="name"
          propertyValue=""
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should render Autocomplete for Icon element with null propertyValue', () => {
      const iconResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Icon,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={iconResource}
          propertyKey="name"
          propertyValue={null}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should render TextPropertyField for non-name property on Icon element', () => {
      const iconResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Icon,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={iconResource}
          propertyKey="size"
          propertyValue={24}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('text-property-field')).toBeInTheDocument();
    });
  });

  describe('Label Property for Non-RichText', () => {
    it('should render TextPropertyField for label property on non-RichText elements', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="label"
          propertyValue="My Label"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('text-property-field')).toBeInTheDocument();
    });
  });

  describe('Text Element align property', () => {
    it('should render a Select dropdown for align property on Text element', () => {
      const textResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Text,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={textResource}
          propertyKey="align"
          propertyValue="left"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should use "left" as the default value when propertyValue is not a string', () => {
      const textResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Text,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={textResource}
          propertyKey="align"
          propertyValue={undefined}
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should render all five align options in the dropdown', () => {
      const textResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Text,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={textResource}
          propertyKey="align"
          propertyValue="center"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // Opening the select reveals the options
      const select = screen.getByRole('combobox');
      fireEvent.mouseDown(select);

      const options = screen.getAllByRole('option');
      expect(options.length).toBe(5);
    });

    it('should call onChange when the align value is changed', () => {
      const textResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.Text,
        config: {},
      } as Resource;

      render(
        <CommonElementPropertyFactory
          resource={textResource}
          propertyKey="align"
          propertyValue="left"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      const select = screen.getByRole('combobox');
      fireEvent.mouseDown(select);

      const centerOption = screen.getAllByRole('option').find((o) => o.getAttribute('data-value') === 'center');
      expect(centerOption).toBeDefined();
      fireEvent.click(centerOption!);
      expect(mockOnChange).toHaveBeenCalledWith('align', 'center', textResource);
    });

    it('should not render align dropdown for non-Text elements', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: ElementTypes.TextInput,
        config: {},
      } as Resource;

      const {container} = render(
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey="align"
          propertyValue="left"
          onChange={mockOnChange}
        />,
        {wrapper: createWrapper()},
      );

      // A non-Text element with align key renders a TextPropertyField, not a Select
      expect(container.querySelector('select')).not.toBeInTheDocument();
      expect(screen.getByTestId('text-property-field')).toBeInTheDocument();
    });
  });
});
