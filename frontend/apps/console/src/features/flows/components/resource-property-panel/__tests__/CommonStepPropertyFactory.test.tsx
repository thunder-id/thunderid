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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ElementTypes} from '../../../models/elements';
import type {Resource} from '../../../models/resources';
import CommonStepPropertyFactory from '../CommonStepPropertyFactory';

// Mock RichTextWithTranslation component
vi.mock('../rich-text/RichTextWithTranslation', () => ({
  default: ({onChange}: {onChange: (html: string) => void}) => (
    <div data-testid="rich-text-with-translation">
      <button type="button" onClick={() => onChange('<p>Updated content</p>')}>
        Rich Text Editor
      </button>
    </div>
  ),
}));

describe('CommonStepPropertyFactory', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('RichText Element with text property', () => {
    it('should render RichTextWithTranslation for text property when resource is RichText', () => {
      const richTextResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.RichText,
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={richTextResource}
          propertyKey="text"
          propertyValue="<p>Test content</p>"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByTestId('rich-text-with-translation')).toBeInTheDocument();
    });

    it('should call onChange when RichText content changes', () => {
      const richTextResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.RichText,
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={richTextResource}
          propertyKey="text"
          propertyValue="<p>Test</p>"
          onChange={mockOnChange}
        />,
      );

      const button = screen.getByText('Rich Text Editor');
      fireEvent.click(button);

      expect(mockOnChange).toHaveBeenCalledWith('text', '<p>Updated content</p>', richTextResource, true);
    });

    it('should not render RichTextWithTranslation for non-text properties on RichText', () => {
      const richTextResource: Resource = {
        id: 'resource-1',
        type: ElementTypes.RichText,
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={richTextResource}
          propertyKey="other"
          propertyValue="test"
          onChange={mockOnChange}
        />,
      );

      expect(screen.queryByTestId('rich-text-with-translation')).not.toBeInTheDocument();
    });
  });

  describe('Boolean Properties', () => {
    it('should render FormControlLabel with Checkbox for boolean true value', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory resource={resource} propertyKey="isEnabled" propertyValue onChange={mockOnChange} />,
      );

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeInTheDocument();
      expect(checkbox).toBeChecked();
    });

    it('should render FormControlLabel with Checkbox for boolean false value', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="isDisabled"
          propertyValue={false}
          onChange={mockOnChange}
        />,
      );

      const checkbox = screen.getByRole('checkbox');
      expect(checkbox).toBeInTheDocument();
      expect(checkbox).not.toBeChecked();
    });

    it('should convert camelCase propertyKey to Start Case label for checkbox', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="showHeader"
          propertyValue
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByText('Show Header')).toBeInTheDocument();
    });

    it('should call onChange with correct parameters when checkbox is toggled', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="active"
          propertyValue={false}
          onChange={mockOnChange}
        />,
      );

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);

      expect(mockOnChange).toHaveBeenCalledWith('active', true, resource);
    });
  });

  describe('String Properties', () => {
    it('should render FormControl with TextField for string value', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="title"
          propertyValue="My Title"
          onChange={mockOnChange}
        />,
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toBeInTheDocument();
      expect(textField).toHaveValue('My Title');
    });

    it('should render TextField for empty string value', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="description"
          propertyValue=""
          onChange={mockOnChange}
        />,
      );

      const textField = screen.getByRole('textbox');
      expect(textField).toBeInTheDocument();
      expect(textField).toHaveValue('');
    });

    it('should convert camelCase propertyKey to Start Case label for text field', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="pageTitle"
          propertyValue="Test"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByText('Page Title')).toBeInTheDocument();
    });

    it('should show placeholder with Start Case property key', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="userName"
          propertyValue=""
          onChange={mockOnChange}
        />,
      );

      const textField = screen.getByPlaceholderText('Enter User Name');
      expect(textField).toBeInTheDocument();
    });

    it('should call onChange when text field value changes', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="name"
          propertyValue="Initial"
          onChange={mockOnChange}
        />,
      );

      const textField = screen.getByRole('textbox');
      fireEvent.change(textField, {target: {value: 'Updated Value'}});

      expect(mockOnChange).toHaveBeenCalledWith('name', 'Updated Value', resource, true);
    });
  });

  describe('Number Properties', () => {
    it('should render FormControl with number input for numeric value', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="maxPerPrompt"
          propertyValue={5}
          onChange={mockOnChange}
        />,
      );

      const numberField = screen.getByRole('spinbutton');
      expect(numberField).toBeInTheDocument();
      expect(numberField).toHaveValue(5);
      expect(screen.getByText('Max Per Prompt')).toBeInTheDocument();
    });

    it('should call onChange with numeric value when number input changes', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="maxPerPrompt"
          propertyValue={5}
          onChange={mockOnChange}
        />,
      );

      const numberField = screen.getByRole('spinbutton');
      fireEvent.change(numberField, {target: {value: '3'}});

      expect(mockOnChange).toHaveBeenCalledWith('maxPerPrompt', 3, resource, true);
    });

    it('should ignore empty numeric input values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="maxPerPrompt"
          propertyValue={5}
          onChange={mockOnChange}
        />,
      );

      const numberField = screen.getByRole('spinbutton');
      fireEvent.change(numberField, {target: {value: ''}});

      expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should use cleaned labels for nested data property keys', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="data.properties.maxPerPrompt"
          propertyValue={5}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByText('Max Per Prompt')).toBeInTheDocument();
      expect(screen.queryByText('Data Properties Max Per Prompt')).not.toBeInTheDocument();
    });
  });

  describe('Null Cases', () => {
    it('should return null for object property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      const {container} = render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="config"
          propertyValue={{nested: 'value'}}
          onChange={mockOnChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });

    it('should return null for array property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      const {container} = render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="items"
          propertyValue={['a', 'b', 'c']}
          onChange={mockOnChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });

    it('should return null for undefined property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      const {container} = render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="undefinedProp"
          propertyValue={undefined}
          onChange={mockOnChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });

    it('should return null for null property values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      const {container} = render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="nullProp"
          propertyValue={null}
          onChange={mockOnChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Additional Props', () => {
    it('should pass additional props to FormControlLabel for boolean values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="enabled"
          propertyValue
          onChange={mockOnChange}
          data-testid="custom-checkbox"
        />,
      );

      expect(screen.getByRole('checkbox')).toBeInTheDocument();
    });

    it('should pass additional props to TextField for string values', () => {
      const resource: Resource = {
        id: 'resource-1',
        type: 'VIEW',
        config: {},
      } as Resource;

      render(
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey="name"
          propertyValue="Test"
          onChange={mockOnChange}
          data-testid="custom-textfield"
        />,
      );

      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });
  });
});
