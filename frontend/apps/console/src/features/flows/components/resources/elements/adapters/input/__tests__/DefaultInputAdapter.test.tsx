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
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import DefaultInputAdapter from '../DefaultInputAdapter';
import type {Element as FlowElement} from '@/features/flows/models/elements';

// Mock dependencies
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

vi.mock('@/features/flows/components/resources/elements/hint', () => ({
  Hint: ({hint}: {hint: string}) => <span data-testid="hint">{hint}</span>,
}));

vi.mock('@/features/flows/components/resources/elements/adapters/PlaceholderComponent', () => ({
  default: ({value}: {value: string}) => <span data-testid="placeholder">{value}</span>,
}));

describe('DefaultInputAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'input-1',
      type: 'TEXT_INPUT',
      category: 'FIELD',
      config: {},
      label: 'Username',
      inputType: 'text',
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render TextField component', () => {
      const resource = createMockElement();

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiTextField-root')).toBeInTheDocument();
    });

    it('should render input element', () => {
      const resource = createMockElement();

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    it('should render label text', () => {
      const resource = createMockElement({label: 'Email Address'});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiInputLabel-root')).toHaveTextContent('Email Address');
    });

    it('should render with full width', () => {
      const resource = createMockElement();

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControl-fullWidth')).toBeInTheDocument();
    });
  });

  describe('Input Types', () => {
    it('should render text input type', () => {
      const resource = createMockElement({inputType: 'text'});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toHaveAttribute('type', 'text');
    });

    it('should render email input type', () => {
      const resource = createMockElement({inputType: 'email'});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toHaveAttribute('type', 'email');
    });

    it('should render password input type with autocomplete off', () => {
      const resource = createMockElement({inputType: 'password'});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      const input = container.querySelector('input');
      expect(input).toHaveAttribute('type', 'password');
      expect(input).toHaveAttribute('autocomplete', 'new-password');
    });
  });

  describe('Placeholder', () => {
    it('should render placeholder when provided', () => {
      const resource = createMockElement({placeholder: 'Enter your username'});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', 'Enter your username');
    });

    it('should render empty placeholder when not provided', () => {
      const resource = createMockElement({placeholder: undefined});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', '');
    });
  });

  describe('Default Value', () => {
    it('should render with default value when provided', () => {
      const resource = createMockElement({defaultValue: 'default text'});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toHaveValue('default text');
    });
  });

  describe('Required Field', () => {
    it('should show required indicator when required is true', () => {
      const resource = createMockElement({required: true});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormLabel-asterisk')).toBeInTheDocument();
    });

    it('should not show required indicator when required is false', () => {
      const resource = createMockElement({required: false});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormLabel-asterisk')).not.toBeInTheDocument();
    });
  });

  describe('Hint Text', () => {
    it('should render hint when provided', () => {
      const resource = createMockElement({hint: 'Enter a valid email'});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByTestId('hint')).toHaveTextContent('Enter a valid email');
    });

    it('should not render hint when not provided', () => {
      const resource = createMockElement({hint: undefined});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });
  });

  describe('Input Constraints', () => {
    it('should set minLength when provided', () => {
      const resource = createMockElement({minLength: 5});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toHaveAttribute('minlength', '5');
    });

    it('should set maxLength when provided', () => {
      const resource = createMockElement({maxLength: 100});

      render(<DefaultInputAdapter resource={resource} />);

      expect(screen.getByRole('textbox')).toHaveAttribute('maxlength', '100');
    });
  });

  describe('Multiline', () => {
    it('should render as multiline when multiline is true', () => {
      const resource = createMockElement({multiline: true});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('textarea')).toBeInTheDocument();
    });

    it('should render as single line when multiline is false', () => {
      const resource = createMockElement({multiline: false});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('textarea')).not.toBeInTheDocument();
    });
  });

  describe('Custom Styling', () => {
    it('should apply className when provided', () => {
      const resource = createMockElement({classes: 'custom-input'});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.custom-input')).toBeInTheDocument();
    });

    it('should apply styles when provided', () => {
      const resource = createMockElement({styles: {marginTop: '10px'}});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      const textField = container.querySelector('.MuiTextField-root');
      expect(textField).toHaveStyle({marginTop: '10px'});
    });
  });

  describe('Empty Label', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiTextField-root')).toBeInTheDocument();
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<DefaultInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiTextField-root')).toBeInTheDocument();
    });
  });
});
