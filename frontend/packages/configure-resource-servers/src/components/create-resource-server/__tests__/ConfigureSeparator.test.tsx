// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import ConfigureSeparator from '../ConfigureSeparator';

describe('ConfigureSeparator', () => {
  it('renders the permission settings and allows enabling hierarchy', () => {
    const onHierarchyChange = vi.fn();
    render(
      <ConfigureSeparator
        delimiter=":"
        hierarchicalPermissions={false}
        onHierarchicalPermissionsChange={onHierarchyChange}
        onDelimiterChange={vi.fn()}
      />,
    );

    expect(screen.getByRole('combobox', {name: /Permission delimiter/})).toBeInTheDocument();
    expect(screen.getByRole('heading', {name: 'Configure permission settings'})).toBeInTheDocument();
    const checkbox = screen.getByRole('checkbox', {name: 'Use hierarchy-aware matching'});
    expect(checkbox).not.toBeChecked();
    fireEvent.click(checkbox);
    expect(onHierarchyChange).toHaveBeenCalledWith(true);
  });

  it('renders the permission preview without a resource server prefix', () => {
    render(
      <ConfigureSeparator
        delimiter=":"
        hierarchicalPermissions={false}
        onHierarchicalPermissionsChange={vi.fn()}
        onDelimiterChange={vi.fn()}
      />,
    );

    expect(screen.getByText('<resource>:<action>')).toBeInTheDocument();
  });

  it('renders preview using dot separator', () => {
    render(
      <ConfigureSeparator
        delimiter="."
        hierarchicalPermissions={false}
        onHierarchicalPermissionsChange={vi.fn()}
        onDelimiterChange={vi.fn()}
      />,
    );

    expect(screen.getByText('<resource>.<action>')).toBeInTheDocument();
  });

  it('calls onReadyChange with true when delimiter is valid', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureSeparator
        delimiter=":"
        hierarchicalPermissions={false}
        onHierarchicalPermissionsChange={vi.fn()}
        onDelimiterChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });
});
