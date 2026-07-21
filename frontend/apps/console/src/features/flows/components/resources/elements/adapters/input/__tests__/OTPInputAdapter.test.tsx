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
import OTPInputAdapter from '../OTPInputAdapter';
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

describe('OTPInputAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'otp-1',
      type: 'OTP_INPUT',
      category: 'FIELD',
      config: {},
      label: 'Enter OTP',
      inputType: 'text',
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render InputLabel component', () => {
      const resource = createMockElement();

      const {container} = render(<OTPInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiInputLabel-root')).toBeInTheDocument();
    });

    it('should render label text', () => {
      const resource = createMockElement({label: 'Verification Code'});

      render(<OTPInputAdapter resource={resource} />);

      expect(screen.getByText('Verification Code')).toBeInTheDocument();
    });

    it('should render 6 input boxes for OTP', () => {
      const resource = createMockElement();

      const {container} = render(<OTPInputAdapter resource={resource} />);

      const inputs = container.querySelectorAll('.MuiOutlinedInput-root');
      expect(inputs).toHaveLength(6);
    });
  });

  describe('Required Field', () => {
    it('should show required indicator when required is true', () => {
      const resource = createMockElement({required: true});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormLabel-asterisk')).toBeInTheDocument();
    });

    it('should not show required indicator when required is false', () => {
      const resource = createMockElement({required: false});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormLabel-asterisk')).not.toBeInTheDocument();
    });
  });

  describe('Hint Text', () => {
    it('should render hint when provided', () => {
      const resource = createMockElement({hint: 'Check your email for the code'});

      render(<OTPInputAdapter resource={resource} />);

      expect(screen.getByTestId('hint')).toHaveTextContent('Check your email for the code');
    });

    it('should not render hint when not provided', () => {
      const resource = createMockElement({hint: undefined});

      render(<OTPInputAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });

    it('should not render hint when empty', () => {
      const resource = createMockElement({hint: ''});

      render(<OTPInputAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });
  });

  describe('Placeholder', () => {
    it('should render placeholder on OTP inputs when provided', () => {
      const resource = createMockElement({placeholder: '0'});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      const inputs = container.querySelectorAll('input');
      inputs.forEach((input) => {
        expect(input).toHaveAttribute('placeholder', '0');
      });
    });

    it('should render empty placeholder when not provided', () => {
      const resource = createMockElement({placeholder: undefined});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      const inputs = container.querySelectorAll('input');
      inputs.forEach((input) => {
        expect(input).toHaveAttribute('placeholder', '');
      });
    });
  });

  describe('Input Type', () => {
    it('should apply input type to OTP fields', () => {
      const resource = createMockElement({inputType: 'number'});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      const inputs = container.querySelectorAll('input');
      inputs.forEach((input) => {
        expect(input).toHaveAttribute('type', 'number');
      });
    });
  });

  describe('Custom Styling', () => {
    it('should apply className when provided', () => {
      const resource = createMockElement({classes: 'custom-otp'});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      expect(container.firstChild).toHaveClass('custom-otp');
    });

    it('should apply styles to inputs when provided', () => {
      const resource = createMockElement({styles: {width: '40px'}});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      const outlinedInputs = container.querySelectorAll('.MuiOutlinedInput-root');
      outlinedInputs.forEach((input) => {
        expect(input).toHaveStyle({width: '40px'});
      });
    });
  });

  describe('Empty Label', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      const label = container.querySelector('.MuiInputLabel-root');
      expect(label).toHaveTextContent('');
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<OTPInputAdapter resource={resource} />);

      const label = container.querySelector('.MuiInputLabel-root');
      expect(label).toHaveTextContent('');
    });
  });
});
