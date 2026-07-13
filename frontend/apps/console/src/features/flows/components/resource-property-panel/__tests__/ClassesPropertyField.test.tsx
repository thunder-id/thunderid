/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import type {Resource} from '../../../models/resources';
import ClassesPropertyField from '../ClassesPropertyField';

vi.mock('react-i18next', () => ({
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

  it('should render a single row for an empty value', () => {
    render(
      <ClassesPropertyField resource={mockResource} propertyKey="className" propertyValue="" onChange={mockOnChange} />,
    );

    expect(screen.getAllByPlaceholderText('flows:core.elements.classesPropertyField.placeholder')).toHaveLength(1);
    // Only row, no delete button should be rendered.
    expect(screen.queryByRole('button', {name: ''})).not.toBeInTheDocument();
  });

  it('should render one row per space-separated class', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn btn-primary"
        onChange={mockOnChange}
      />,
    );

    const inputs = screen.getAllByPlaceholderText('flows:core.elements.classesPropertyField.placeholder');
    expect(inputs).toHaveLength(2);
    expect(inputs[0]).toHaveValue('btn');
    expect(inputs[1]).toHaveValue('btn-primary');
  });

  it('should add a new empty row when the add button is clicked', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn"
        onChange={mockOnChange}
      />,
    );

    fireEvent.click(screen.getByText('flows:core.elements.classesPropertyField.addClass'));

    expect(screen.getAllByPlaceholderText('flows:core.elements.classesPropertyField.placeholder')).toHaveLength(2);
    expect(mockOnChange).toHaveBeenCalledWith('className', 'btn ', mockResource, undefined);
  });

  it('should update a row value on change and debounce the update', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn"
        onChange={mockOnChange}
      />,
    );

    const input = screen.getByPlaceholderText('flows:core.elements.classesPropertyField.placeholder');
    fireEvent.change(input, {target: {value: 'btn-secondary'}});

    expect(mockOnChange).toHaveBeenCalledWith('className', 'btn-secondary', mockResource, true);
  });

  it('should remove a row when its delete button is clicked', () => {
    render(
      <ClassesPropertyField
        resource={mockResource}
        propertyKey="className"
        propertyValue="btn btn-primary"
        onChange={mockOnChange}
      />,
    );

    const deleteButtons = screen.getAllByRole('button', {name: 'common:actions.delete'});
    expect(deleteButtons).toHaveLength(2);

    fireEvent.click(deleteButtons[0]);

    expect(mockOnChange).toHaveBeenCalledWith('className', 'btn-primary', mockResource, undefined);
    expect(screen.getAllByPlaceholderText('flows:core.elements.classesPropertyField.placeholder')).toHaveLength(1);
  });
});
