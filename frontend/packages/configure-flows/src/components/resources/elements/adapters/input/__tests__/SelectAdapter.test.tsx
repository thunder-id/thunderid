// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Element as FlowElement} from '../../../../../../models/elements';
import SelectAdapter from '../SelectAdapter';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: (value: string | undefined) => value,
  }),
}));

vi.mock('../../../hint', () => ({
  Hint: ({hint}: {hint: string}) => <span data-testid="hint">{hint}</span>,
}));

describe('SelectAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'select-1',
      type: 'SELECT',
      category: 'FIELD',
      config: {},
      label: 'Country',
      placeholder: 'Select a country',
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the select component', () => {
      const resource = createMockElement();

      const {container} = render(<SelectAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControl-root')).toBeInTheDocument();
    });

    it('should render the label', () => {
      const resource = createMockElement({label: 'Country'});

      render(<SelectAdapter resource={resource} />);

      expect(screen.getByText('Country')).toBeInTheDocument();
    });

    it('should render the placeholder as a disabled menu item', () => {
      const resource = createMockElement({placeholder: 'Choose one'});

      render(<SelectAdapter resource={resource} />);

      expect(screen.getByText('Choose one')).toBeInTheDocument();
    });

    it('should render required indicator when required is true', () => {
      const resource = createMockElement({required: true});

      const {container} = render(<SelectAdapter resource={resource} />);

      // MUI adds an asterisk span with class MuiFormLabel-asterisk for required fields
      const asterisk = container.querySelector('.MuiFormLabel-asterisk');
      expect(asterisk).toBeInTheDocument();
    });
  });

  describe('Hint', () => {
    it('should render hint when provided', () => {
      const resource = createMockElement({hint: 'Select your country of residence'});

      render(<SelectAdapter resource={resource} />);

      expect(screen.getByTestId('hint')).toHaveTextContent('Select your country of residence');
    });

    it('should not render hint when not provided', () => {
      const resource = createMockElement({hint: undefined});

      render(<SelectAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });
  });

  describe('Default Values', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<SelectAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControl-root')).toBeInTheDocument();
    });

    it('should handle empty placeholder', () => {
      const resource = createMockElement({placeholder: undefined});

      const {container} = render(<SelectAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControl-root')).toBeInTheDocument();
    });
  });
});
