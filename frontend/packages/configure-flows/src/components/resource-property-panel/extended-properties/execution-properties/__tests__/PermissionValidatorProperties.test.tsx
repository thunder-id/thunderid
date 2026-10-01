// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import PermissionValidatorProperties from '../PermissionValidatorProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.permissionValidator.description': 'Configure the required scopes.',
        'flows:core.executions.permissionValidator.requiredScopes.label': 'Required scopes',
        'flows:core.executions.permissionValidator.requiredScopes.placeholder': 'read, write',
        'flows:core.executions.permissionValidator.requiredScopes.hint': 'Comma-separated scope names.',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('PermissionValidatorProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and field label', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<PermissionValidatorProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure the required scopes.')).toBeInTheDocument();
    expect(screen.getByText('Required scopes')).toBeInTheDocument();
  });

  it('should render an empty field when the node has no required scopes', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<PermissionValidatorProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Required scopes')).toHaveValue('');
  });

  it('should join the configured required scopes into a comma-separated field', () => {
    const resource = {data: {properties: {requiredScopes: ['read', 'write']}}} as unknown as Resource;

    render(<PermissionValidatorProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByLabelText('Required scopes')).toHaveValue('read, write');
  });

  it('should commit the parsed list on blur', () => {
    const resource = {data: {properties: {}}} as unknown as Resource;

    render(<PermissionValidatorProperties resource={resource} onChange={mockOnChange} />);

    const input = screen.getByLabelText('Required scopes');
    fireEvent.change(input, {target: {value: 'read, write'}});
    fireEvent.blur(input);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.requiredScopes', ['read', 'write'], resource);
  });
});
