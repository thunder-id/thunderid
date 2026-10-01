// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../models/resources';
import ButtonExtendedProperties from '../ButtonExtendedProperties';

// Mock dependencies
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('ButtonExtendedProperties', () => {
  const mockOnChange = vi.fn();

  const createMockResource = (overrides: Partial<Resource> = {}): Resource =>
    ({
      id: 'button-1',
      type: 'ACTION',
      category: 'ACTION',
      resourceType: 'ELEMENT',
      ...overrides,
    }) as Resource;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the start icon label', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.buttonExtendedProperties.startIcon.label')).toBeInTheDocument();
    });

    it('should render the end icon label', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.buttonExtendedProperties.endIcon.label')).toBeInTheDocument();
    });

    it('should render start icon input field', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const startIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.startIcon.placeholder');
      expect(startIconInput).toBeInTheDocument();
    });

    it('should render end icon input field', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const endIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.endIcon.placeholder');
      expect(endIconInput).toBeInTheDocument();
    });

    it('should render hint text for start icon', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.buttonExtendedProperties.startIcon.hint')).toBeInTheDocument();
    });

    it('should render hint text for end icon', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.buttonExtendedProperties.endIcon.hint')).toBeInTheDocument();
    });

    it('should render dividers', () => {
      const resource = createMockResource();

      const {container} = render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const dividers = container.querySelectorAll('.MuiDivider-root');
      expect(dividers.length).toBe(2);
    });
  });

  describe('Initial Values', () => {
    it('should display existing startIcon value', () => {
      const resource = createMockResource({
        startIcon: '/assets/icons/test-start.svg',
      } as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const startIconInput = screen.getByPlaceholderText<HTMLInputElement>(
        'flows:core.buttonExtendedProperties.startIcon.placeholder',
      );
      expect(startIconInput.value).toBe('/assets/icons/test-start.svg');
    });

    it('should display existing endIcon value', () => {
      const resource = createMockResource({
        endIcon: '/assets/icons/test-end.svg',
      } as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const endIconInput = screen.getByPlaceholderText<HTMLInputElement>(
        'flows:core.buttonExtendedProperties.endIcon.placeholder',
      );
      expect(endIconInput.value).toBe('/assets/icons/test-end.svg');
    });

    it('should display empty value when startIcon is not set', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const startIconInput = screen.getByPlaceholderText<HTMLInputElement>(
        'flows:core.buttonExtendedProperties.startIcon.placeholder',
      );
      expect(startIconInput.value).toBe('');
    });

    it('should display empty value when endIcon is not set', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const endIconInput = screen.getByPlaceholderText<HTMLInputElement>(
        'flows:core.buttonExtendedProperties.endIcon.placeholder',
      );
      expect(endIconInput.value).toBe('');
    });
  });

  describe('Change Handlers', () => {
    it('should call onChange when start icon value changes', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const startIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.startIcon.placeholder');
      fireEvent.change(startIconInput, {target: {value: '/new/icon/path.svg'}});

      expect(mockOnChange).toHaveBeenCalledWith('startIcon', '/new/icon/path.svg', resource, true);
    });

    it('should call onChange when end icon value changes', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const endIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.endIcon.placeholder');
      fireEvent.change(endIconInput, {target: {value: '/new/end/icon.svg'}});

      expect(mockOnChange).toHaveBeenCalledWith('endIcon', '/new/end/icon.svg', resource, true);
    });

    it('should call onChange with empty string when clearing start icon', () => {
      const resource = createMockResource({
        startIcon: '/existing/icon.svg',
      } as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const startIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.startIcon.placeholder');
      fireEvent.change(startIconInput, {target: {value: ''}});

      expect(mockOnChange).toHaveBeenCalledWith('startIcon', '', resource, true);
    });

    it('should call onChange with empty string when clearing end icon', () => {
      const resource = createMockResource({
        endIcon: '/existing/icon.svg',
      } as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const endIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.endIcon.placeholder');
      fireEvent.change(endIconInput, {target: {value: ''}});

      expect(mockOnChange).toHaveBeenCalledWith('endIcon', '', resource, true);
    });
  });

  describe('Action', () => {
    it('should render the action label', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.buttonExtendedProperties.action.label')).toBeInTheDocument();
    });

    it('should default to TRIGGER when eventType is not set', () => {
      const resource = createMockResource();

      const {container} = render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const select = container.querySelector('#event-type-select');
      expect(select).toHaveTextContent('flows:core.buttonExtendedProperties.action.trigger');
    });

    it('should display SUBMIT when eventType is SUBMIT', () => {
      const resource = createMockResource({eventType: 'SUBMIT'} as Partial<Resource>);

      const {container} = render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const select = container.querySelector('#event-type-select');
      expect(select).toHaveTextContent('flows:core.buttonExtendedProperties.action.submit');
    });

    it('should display Confirm when the button carries the confirm action', () => {
      // A confirmation button is a submit button plus the prompt action type, so
      // the action type has to win over the plain event type.
      const resource = createMockResource({
        actionType: 'CONFIRM',
        eventType: 'SUBMIT',
      } as Partial<Resource>);

      const {container} = render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const select = container.querySelector('#event-type-select');
      expect(select).toHaveTextContent('flows:core.buttonExtendedProperties.action.confirm');
    });

    it('should write both the event type and the action type when Confirm is picked', async () => {
      const user = userEvent.setup();
      const resource = createMockResource({eventType: 'TRIGGER'} as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox'));
      await user.click(screen.getByRole('option', {name: 'flows:core.buttonExtendedProperties.action.confirm'}));

      expect(mockOnChange).toHaveBeenCalledWith('eventType', 'SUBMIT', resource);
      expect(mockOnChange).toHaveBeenCalledWith('actionType', 'CONFIRM', resource);
    });

    it('should clear the action type when moving from Confirm back to a plain action', async () => {
      const user = userEvent.setup();
      const resource = createMockResource({
        actionType: 'CONFIRM',
        eventType: 'SUBMIT',
      } as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox'));
      await user.click(screen.getByRole('option', {name: 'flows:core.buttonExtendedProperties.action.trigger'}));

      expect(mockOnChange).toHaveBeenCalledWith('eventType', 'TRIGGER', resource);
      expect(mockOnChange).toHaveBeenCalledWith('actionType', '', resource);
    });

    it('should preserve an action type the selector does not model', async () => {
      // REJECT is a valid prompt action type with no option here. Clearing it would silently
      // discard a type authored directly in the flow definition.
      const user = userEvent.setup();
      const resource = createMockResource({
        actionType: 'REJECT',
        eventType: 'SUBMIT',
      } as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox'));
      await user.click(screen.getByRole('option', {name: 'flows:core.buttonExtendedProperties.action.trigger'}));

      expect(mockOnChange).toHaveBeenCalledWith('eventType', 'TRIGGER', resource);
      expect(mockOnChange).not.toHaveBeenCalledWith('actionType', '', resource);
    });

    it('should not clear the action type when it was never set', async () => {
      const user = userEvent.setup();
      const resource = createMockResource({eventType: 'TRIGGER'} as Partial<Resource>);

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox'));
      await user.click(screen.getByRole('option', {name: 'flows:core.buttonExtendedProperties.action.submit'}));

      expect(mockOnChange).toHaveBeenCalledWith('eventType', 'SUBMIT', resource);
      expect(mockOnChange).not.toHaveBeenCalledWith('actionType', '', resource);
    });
  });

  describe('Input Attributes', () => {
    it('should have correct id for start icon input', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const startIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.startIcon.placeholder');
      expect(startIconInput).toHaveAttribute('id', 'start-icon-input');
    });

    it('should have correct id for end icon input', () => {
      const resource = createMockResource();

      render(<ButtonExtendedProperties resource={resource} onChange={mockOnChange} />);

      const endIconInput = screen.getByPlaceholderText('flows:core.buttonExtendedProperties.endIcon.placeholder');
      expect(endIconInput).toHaveAttribute('id', 'end-icon-input');
    });
  });
});
