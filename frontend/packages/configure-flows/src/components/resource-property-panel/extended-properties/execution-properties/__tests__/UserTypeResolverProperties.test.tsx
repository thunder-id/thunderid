// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import UserTypeResolverProperties from '../UserTypeResolverProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.userTypeResolver.description': 'Configure which user types are allowed.',
        'flows:core.executions.userTypeResolver.allowedUserTypes.label': 'Allowed user types',
        'flows:core.executions.userTypeResolver.allowedUserTypes.placeholder': 'employee, customer',
        'flows:core.executions.userTypeResolver.allowedUserTypes.hint': 'Comma-separated user type ids.',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('UserTypeResolverProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and field label', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<UserTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure which user types are allowed.')).toBeInTheDocument();
    expect(screen.getByText('Allowed user types')).toBeInTheDocument();
  });

  it('should render an empty field when the node has no allowed user types', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<UserTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Allowed user types')).toHaveValue('');
  });

  it('should join the configured allowed user types into a comma-separated field', () => {
    const resource = {data: {properties: {allowedUserTypes: ['employee', 'customer']}}} as unknown as Resource;

    render(<UserTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Allowed user types')).toHaveValue('employee, customer');
  });

  it('should commit the parsed list on blur', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<UserTypeResolverProperties resource={resource} onChange={mockOnChange} />);

    const input = screen.getByLabelText('Allowed user types');
    fireEvent.change(input, {target: {value: 'employee, customer'}});
    fireEvent.blur(input);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.allowedUserTypes', ['employee', 'customer'], resource);
  });
});
