// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {FieldOption} from '../../../../../models/base';
import type {Element as FlowElement} from '../../../../../models/elements';
import ChoiceAdapter from '../ChoiceAdapter';

// Mock the Hint component
vi.mock('../../hint', () => ({
  Hint: ({hint}: {hint: string}) => <span data-testid="hint">{hint}</span>,
}));

describe('ChoiceAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'choice-1',
      type: 'DROPDOWN',
      category: 'FIELD',
      config: {},
      label: 'Select an option',
      ...overrides,
    }) as FlowElement;

  const createMockOptions = (): FieldOption[] => [
    {key: 'opt1', value: 'option1', label: 'Option 1'},
    {key: 'opt2', value: 'option2', label: 'Option 2'},
    {key: 'opt3', value: 'option3', label: 'Option 3'},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render FormControl container', () => {
      const resource = createMockElement();

      const {container} = render(<ChoiceAdapter resource={resource} />);

      expect(container.querySelector('.MuiFormControl-root')).toBeInTheDocument();
    });

    it('should render form label with resource label', () => {
      const resource = createMockElement({label: 'Choose your preference'});

      render(<ChoiceAdapter resource={resource} />);

      expect(screen.getByText('Choose your preference')).toBeInTheDocument();
    });

    it('should render RadioGroup component', () => {
      const resource = createMockElement();

      const {container} = render(<ChoiceAdapter resource={resource} />);

      expect(container.querySelector('.MuiRadioGroup-root')).toBeInTheDocument();
    });
  });

  describe('Options Rendering', () => {
    it('should render radio options for each field option', () => {
      const options = createMockOptions();
      const resource = createMockElement({options});

      render(<ChoiceAdapter resource={resource} />);

      expect(screen.getByText('Option 1')).toBeInTheDocument();
      expect(screen.getByText('Option 2')).toBeInTheDocument();
      expect(screen.getByText('Option 3')).toBeInTheDocument();
    });

    it('should render radio buttons for each option', () => {
      const options = createMockOptions();
      const resource = createMockElement({options});

      render(<ChoiceAdapter resource={resource} />);

      const radioButtons = screen.getAllByRole('radio');
      expect(radioButtons).toHaveLength(3);
    });

    it('should handle empty options array', () => {
      const resource = createMockElement({options: []});

      const {container} = render(<ChoiceAdapter resource={resource} />);

      const radioButtons = container.querySelectorAll('input[type="radio"]');
      expect(radioButtons).toHaveLength(0);
    });

    it('should handle undefined options', () => {
      const resource = createMockElement({options: undefined});

      const {container} = render(<ChoiceAdapter resource={resource} />);

      const radioButtons = container.querySelectorAll('input[type="radio"]');
      expect(radioButtons).toHaveLength(0);
    });
  });

  describe('Default Value', () => {
    it('should set default value on RadioGroup', () => {
      const options = createMockOptions();
      const resource = createMockElement({
        options,
        defaultValue: 'option2',
      });

      render(<ChoiceAdapter resource={resource} />);

      const radioButtons = screen.getAllByRole('radio');
      expect(radioButtons[1]).toBeChecked();
    });

    it('should handle no default value', () => {
      const options = createMockOptions();
      const resource = createMockElement({
        options,
        defaultValue: undefined,
      });

      render(<ChoiceAdapter resource={resource} />);

      const radioButtons = screen.getAllByRole('radio');
      radioButtons.forEach((radio) => {
        expect(radio).not.toBeChecked();
      });
    });
  });

  describe('Hint Text', () => {
    it('should render hint when provided', () => {
      const resource = createMockElement({hint: 'Select one of the options above'});

      render(<ChoiceAdapter resource={resource} />);

      expect(screen.getByTestId('hint')).toBeInTheDocument();
      expect(screen.getByTestId('hint')).toHaveTextContent('Select one of the options above');
    });

    it('should not render hint when not provided', () => {
      const resource = createMockElement({hint: undefined});

      render(<ChoiceAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });

    it('should not render hint when empty string', () => {
      const resource = createMockElement({hint: ''});

      render(<ChoiceAdapter resource={resource} />);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });
  });

  describe('Form Label', () => {
    it('should render with resource id as label id', () => {
      const resource = createMockElement({id: 'my-choice-field'});

      const {container} = render(<ChoiceAdapter resource={resource} />);

      const label = container.querySelector('.MuiFormLabel-root');
      expect(label).toHaveAttribute('id', 'my-choice-field');
    });

    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<ChoiceAdapter resource={resource} />);

      const label = container.querySelector('.MuiFormLabel-root');
      expect(label).toBeInTheDocument();
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<ChoiceAdapter resource={resource} />);

      const label = container.querySelector('.MuiFormLabel-root');
      expect(label).toBeInTheDocument();
    });
  });
});
