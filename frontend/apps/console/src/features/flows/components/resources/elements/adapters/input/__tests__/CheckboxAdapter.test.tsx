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
import CheckboxAdapter from '../CheckboxAdapter';
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

describe('CheckboxAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'checkbox-1',
      type: 'CHECKBOX',
      category: 'FIELD',
      config: {},
      label: 'Accept terms and conditions',
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render FormControlLabel component', () => {
      const resource = createMockElement();

      const {container} = render(<CheckboxAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControlLabel-root')).toBeInTheDocument();
    });

    it('should render Checkbox component', () => {
      const resource = createMockElement();

      render(<CheckboxAdapter resource={resource} />);

      expect(screen.getByRole('checkbox')).toBeInTheDocument();
    });

    it('should render label text', () => {
      const resource = createMockElement({label: 'I agree'});

      render(<CheckboxAdapter resource={resource} />);

      expect(screen.getByText('I agree')).toBeInTheDocument();
    });

    it('should render checkbox as checked by default', () => {
      const resource = createMockElement();

      render(<CheckboxAdapter resource={resource} />);

      expect(screen.getByRole('checkbox')).toBeChecked();
    });
  });

  describe('Required Field', () => {
    it('should show required indicator when required is true', () => {
      const resource = createMockElement({required: true});

      const {container} = render(<CheckboxAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControlLabel-asterisk')).toBeInTheDocument();
    });

    it('should not show required indicator when required is false', () => {
      const resource = createMockElement({required: false});

      const {container} = render(<CheckboxAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControlLabel-asterisk')).not.toBeInTheDocument();
    });
  });

  describe('Hint Text', () => {
    it('should render hint when provided', () => {
      const resource = createMockElement({hint: 'Please read the terms'});

      render(<CheckboxAdapter resource={resource} />);

      expect(screen.getByTestId('hint')).toHaveTextContent('Please read the terms');
    });

    it('should not render hint when not provided', () => {
      const resource = createMockElement({hint: undefined});

      render(<CheckboxAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });

    it('should not render hint when empty', () => {
      const resource = createMockElement({hint: ''});

      render(<CheckboxAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });
  });

  describe('Custom Styling', () => {
    it('should apply className when provided', () => {
      const resource = createMockElement({classes: 'custom-checkbox'});

      const {container} = render(<CheckboxAdapter resource={resource} />);

      expect(container.querySelector('.custom-checkbox')).toBeInTheDocument();
    });

    it('should apply styles when provided', () => {
      const resource = createMockElement({styles: {marginTop: '10px'}});

      const {container} = render(<CheckboxAdapter resource={resource} />);

      const label = container.querySelector('.MuiFormControlLabel-root');
      expect(label).toHaveStyle({marginTop: '10px'});
    });
  });

  describe('Empty Label', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<CheckboxAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControlLabel-label')).toHaveTextContent('');
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<CheckboxAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControlLabel-label')).toHaveTextContent('');
    });
  });
});
