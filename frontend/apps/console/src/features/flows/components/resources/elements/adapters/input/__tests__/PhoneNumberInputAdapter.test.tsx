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
import PhoneNumberInputAdapter from '../PhoneNumberInputAdapter';
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

describe('PhoneNumberInputAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'phone-1',
      type: 'PHONE_INPUT',
      category: 'FIELD',
      config: {},
      label: 'Phone Number',
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render TextField component', () => {
      const resource = createMockElement();

      const {container} = render(<PhoneNumberInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiTextField-root')).toBeInTheDocument();
    });

    it('should render input with type number', () => {
      const resource = createMockElement();

      render(<PhoneNumberInputAdapter resource={resource} />);

      expect(screen.getByRole('spinbutton')).toHaveAttribute('type', 'number');
    });

    it('should render with label', () => {
      const resource = createMockElement({label: 'Mobile Number'});

      render(<PhoneNumberInputAdapter resource={resource} />);

      expect(screen.getByLabelText('Mobile Number')).toBeInTheDocument();
    });
  });

  describe('Placeholder', () => {
    it('should render placeholder when provided', () => {
      const resource = createMockElement({placeholder: '+1 (555) 123-4567'});

      render(<PhoneNumberInputAdapter resource={resource} />);

      expect(screen.getByRole('spinbutton')).toHaveAttribute('placeholder', '+1 (555) 123-4567');
    });

    it('should render empty placeholder when not provided', () => {
      const resource = createMockElement({placeholder: undefined});

      render(<PhoneNumberInputAdapter resource={resource} />);

      expect(screen.getByRole('spinbutton')).toHaveAttribute('placeholder', '');
    });
  });

  describe('Required Field', () => {
    it('should show required indicator when required is true', () => {
      const resource = createMockElement({required: true});

      const {container} = render(<PhoneNumberInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormLabel-asterisk')).toBeInTheDocument();
    });

    it('should not show required indicator when required is false', () => {
      const resource = createMockElement({required: false});

      const {container} = render(<PhoneNumberInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormLabel-asterisk')).not.toBeInTheDocument();
    });
  });

  describe('Hint Text', () => {
    it('should render hint when provided', () => {
      const resource = createMockElement({hint: 'Include country code'});

      render(<PhoneNumberInputAdapter resource={resource} />);

      expect(screen.getByTestId('hint')).toHaveTextContent('Include country code');
    });

    it('should not render hint when not provided', () => {
      const resource = createMockElement({hint: undefined});

      render(<PhoneNumberInputAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });

    it('should not render hint when empty', () => {
      const resource = createMockElement({hint: ''});

      render(<PhoneNumberInputAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });
  });

  describe('Custom Styling', () => {
    it('should apply className when provided', () => {
      const resource = createMockElement({classes: 'custom-phone'});

      const {container} = render(<PhoneNumberInputAdapter resource={resource} />);

      expect(container.querySelector('.custom-phone')).toBeInTheDocument();
    });
  });

  describe('Empty Label', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<PhoneNumberInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiTextField-root')).toBeInTheDocument();
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<PhoneNumberInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiTextField-root')).toBeInTheDocument();
    });
  });
});
