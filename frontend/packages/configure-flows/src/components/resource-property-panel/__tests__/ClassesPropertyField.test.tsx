// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../models/resources';
import ClassesPropertyField from '../ClassesPropertyField';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('ClassesPropertyField', () => {
  const mockOnChange = vi.fn();

  const mockResource: Resource = {
    id: 'resource-1',
    type: 'BUTTON',
    config: {},
  } as Resource;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should show only the placeholder when there are no classes', () => {
    render(
      <ClassesPropertyField resource={mockResource} propertyKey="className" propertyValue="" onChange={mockOnChange} />,
    );

    expect(screen.getByPlaceholderText('flows:core.elements.classesPropertyField.placeholder')).toBeInTheDocument();
    expect(screen.queryAllByRole('button')).toHaveLength(0);
  });

  it('should render a chip per space-separated class', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn btn-primary"
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByText('btn')).toBeInTheDocument();
    expect(screen.getByText('btn-primary')).toBeInTheDocument();
    // With chips present the placeholder is dropped.
    expect(
      screen.queryByPlaceholderText('flows:core.elements.classesPropertyField.placeholder'),
    ).not.toBeInTheDocument();
  });

  it('should append a class when one is typed and committed', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn"
        onChange={mockOnChange}
      />,
    );

    const input = screen.getByRole('combobox');
    fireEvent.change(input, {target: {value: 'btn-primary'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(mockOnChange).toHaveBeenCalledWith('className', 'btn btn-primary', mockResource);
  });

  it('should split a pasted value containing whitespace into separate classes', () => {
    render(
      <ClassesPropertyField resource={mockResource} propertyKey="className" propertyValue="" onChange={mockOnChange} />,
    );

    const input = screen.getByRole('combobox');
    fireEvent.change(input, {target: {value: 'btn btn-primary'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(mockOnChange).toHaveBeenCalledWith('className', 'btn btn-primary', mockResource);
  });

  it('should not add the same class twice', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn"
        onChange={mockOnChange}
      />,
    );

    const input = screen.getByRole('combobox');
    fireEvent.change(input, {target: {value: 'btn'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    // Whatever the widget emits, the persisted value never gains a duplicate.
    const persisted: string[] = mockOnChange.mock.calls.map((call) => call[1] as string);
    expect(persisted.every((value: string) => value === 'btn')).toBe(true);
    expect(screen.getAllByText('btn')).toHaveLength(1);
  });

  it('should remove a class when its chip is deleted', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn btn-primary"
        onChange={mockOnChange}
      />,
    );

    // Each chip renders a delete button; the first belongs to "btn".
    fireEvent.click(screen.getAllByTestId('CancelIcon')[0]);

    expect(mockOnChange).toHaveBeenCalledWith('className', 'btn-primary', mockResource);
  });
});
