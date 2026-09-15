// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FieldExtendedProperties from '../FieldExtendedProperties';
import {ElementTypes} from '@/features/flows/models/elements';
import type {Resource} from '@/features/flows/models/resources';

// Mock dependencies
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

interface MockAttribute {
  attribute: string;
  credential: boolean;
  userTypes: string[];
}

const mockUseGetUserTypeAttributes = vi.fn<() => {attributes: MockAttribute[]; isLoading: boolean}>();

vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserTypeAttributes: (): {attributes: MockAttribute[]; isLoading: boolean} => mockUseGetUserTypeAttributes(),
}));

const mockHasResourceFieldNotification = vi.fn().mockReturnValue(false);
const mockGetResourceFieldNotification = vi.fn().mockReturnValue('');

vi.mock('@/features/flows/hooks/useValidationStatus', () => ({
  default: () => ({
    selectedNotification: {
      hasResourceFieldNotification: mockHasResourceFieldNotification,
      getResourceFieldNotification: mockGetResourceFieldNotification,
    },
  }),
}));

describe('FieldExtendedProperties', () => {
  const mockOnChange = vi.fn();

  const createMockResource = (type: string, overrides: Partial<Resource> = {}): Resource =>
    ({
      id: 'field-1',
      type,
      category: 'FIELD',
      resourceType: 'ELEMENT',
      ...overrides,
    }) as Resource;

  beforeEach(() => {
    vi.clearAllMocks();
    mockHasResourceFieldNotification.mockReturnValue(false);
    mockGetResourceFieldNotification.mockReturnValue('');
    mockUseGetUserTypeAttributes.mockReturnValue({
      attributes: [
        {attribute: 'department', credential: false, userTypes: ['Employee']},
        {attribute: 'email', credential: false, userTypes: ['Person', 'Employee']},
        {attribute: 'password', credential: true, userTypes: ['Person']},
        {attribute: 'username', credential: false, userTypes: ['Person']},
      ],
      isLoading: false,
    });
  });

  describe('Rendering', () => {
    it('should render the component for text input', () => {
      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.fieldExtendedProperties.attribute')).toBeInTheDocument();
    });

    it('should render Autocomplete component', () => {
      const resource = createMockResource(ElementTypes.TextInput);

      const {container} = render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(container.querySelector('.MuiAutocomplete-root')).toBeInTheDocument();
    });

    it('should render with placeholder text', () => {
      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByPlaceholderText('flows:core.fieldExtendedProperties.selectAttribute')).toBeInTheDocument();
    });
  });

  describe('Password Input Handling', () => {
    it('should list only credential attributes for PasswordInput type', async () => {
      const user = userEvent.setup();
      const resource = createMockResource(ElementTypes.PasswordInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      const input = screen.getByRole('combobox');
      expect(input).toBeInTheDocument();

      await user.click(input);

      expect(await screen.findByRole('option', {name: /password/})).toBeInTheDocument();
      expect(screen.queryByRole('option', {name: /email/})).not.toBeInTheDocument();
    });

    it('should accept a custom value for PasswordInput type', () => {
      const resource = createMockResource(ElementTypes.PasswordInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByRole('combobox'), {target: {value: 'customCredential'}});

      expect(mockOnChange).toHaveBeenCalledWith('ref', 'customCredential', resource, true);
    });
  });

  describe('Attribute Selection', () => {
    it('should list non credential attributes aggregated across user types', async () => {
      const user = userEvent.setup();
      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox'));

      expect(await screen.findByRole('option', {name: /department/})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: /email/})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: /username/})).toBeInTheDocument();
      expect(screen.queryByRole('option', {name: /password/})).not.toBeInTheDocument();
    });

    // Regression: feeding typed text back in as the Autocomplete value makes MUI treat the input as
    // a pristine selection and stop filtering, leaving the whole list showing while typing.
    it('should filter options by the typed text', () => {
      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByRole('combobox'), {target: {value: 'dep'}});

      const options = screen.queryAllByRole('option');
      expect(options).toHaveLength(1);
      expect(options[0]).toHaveTextContent('department');
    });

    it('should annotate options with the user types declaring them', async () => {
      const user = userEvent.setup();
      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox'));

      expect(await screen.findByRole('option', {name: /email/})).toHaveTextContent('Person, Employee');
    });

    it('should display current ref value', () => {
      const resource = createMockResource(ElementTypes.TextInput, {ref: 'email'} as Partial<Resource>);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      const input = screen.getByRole('combobox');
      expect(input).toHaveValue('email');
    });
  });

  describe('Resource Change Handling', () => {
    it('should sync value when resource changes', () => {
      const resource1 = createMockResource(ElementTypes.TextInput, {id: 'field-1', ref: 'email'} as Partial<Resource>);
      const resource2 = createMockResource(ElementTypes.TextInput, {
        id: 'field-2',
        ref: 'username',
      } as Partial<Resource>);

      const {rerender} = render(<FieldExtendedProperties resource={resource1} onChange={mockOnChange} />);

      let input = screen.getByRole('combobox');
      expect(input).toHaveValue('email');

      rerender(<FieldExtendedProperties resource={resource2} onChange={mockOnChange} />);

      input = screen.getByRole('combobox');
      expect(input).toHaveValue('username');
    });

    it('should sync to empty when resource ref changes to undefined (same id)', () => {
      const resourceWithRef = createMockResource(ElementTypes.TextInput, {
        id: 'field-1',
        ref: 'email',
      } as Partial<Resource>);
      const resourceWithoutRef = createMockResource(ElementTypes.TextInput, {id: 'field-1'} as Partial<Resource>);

      const {rerender} = render(<FieldExtendedProperties resource={resourceWithRef} onChange={mockOnChange} />);

      let input = screen.getByRole('combobox');
      expect(input).toHaveValue('email');

      rerender(<FieldExtendedProperties resource={resourceWithoutRef} onChange={mockOnChange} />);

      input = screen.getByRole('combobox');
      expect(input).toHaveValue('');
    });
  });

  describe('onChange Handling', () => {
    it('should call onChange when selecting an attribute from dropdown', async () => {
      const user = userEvent.setup();
      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      const input = screen.getByRole('combobox');
      await user.click(input);

      // Wait for dropdown to open and select an option
      const option = await screen.findByRole('option', {name: /email/});
      await user.click(option);

      expect(mockOnChange).toHaveBeenCalledWith('ref', 'email', resource);
    });

    it('should call onChange with empty string when clearing selection', () => {
      const resource = createMockResource(ElementTypes.TextInput, {ref: 'email'} as Partial<Resource>);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      // Clear the input by clicking the clear button
      const clearButton = screen.getByTitle('Clear');
      fireEvent.click(clearButton);

      expect(mockOnChange).toHaveBeenCalledWith('ref', '', resource);
    });

    it('should call onChange when typing a custom value (free-solo)', () => {
      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      const input = screen.getByRole('combobox');
      fireEvent.change(input, {target: {value: 'customAttribute'}});

      expect(mockOnChange).toHaveBeenCalledWith('ref', 'customAttribute', resource, true);
    });
  });

  describe('Error Message Handling', () => {
    it('should display error message when validation error exists', () => {
      mockHasResourceFieldNotification.mockReturnValue(true);
      mockGetResourceFieldNotification.mockReturnValue('This field is required');

      const resource = createMockResource(ElementTypes.TextInput);

      render(<FieldExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByText('This field is required')).toBeInTheDocument();
    });
  });
});
