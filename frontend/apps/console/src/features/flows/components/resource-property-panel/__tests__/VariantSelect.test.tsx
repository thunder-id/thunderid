// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import {ValidationContext, type ValidationContextProps} from '../../../context/ValidationContext';
import type {Element} from '../../../models/elements';
import Notification from '../../../models/notification';
import type {Resource} from '../../../models/resources';
import VariantSelect from '../VariantSelect';

describe('VariantSelect', () => {
  const createResource = (variants?: Element[]): Resource =>
    ({
      id: 'resource-1',
      type: 'ACTION',
      category: 'ACTION',
      variants,
    }) as unknown as Resource;

  const mockVariants: Element[] = [
    {variant: 'PRIMARY'} as unknown as Element,
    {variant: 'SECONDARY'} as unknown as Element,
    {variant: 'TEXT'} as unknown as Element,
  ];

  const defaultContextValue: ValidationContextProps = {
    isValid: true,
    notifications: [],
    getNotification: vi.fn(),
  };

  const createWrapper = (contextValue: ValidationContextProps = defaultContextValue) => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ValidationContext.Provider value={contextValue}>{children}</ValidationContext.Provider>;
    }
    return Wrapper;
  };

  it('should return null when resource has no variants', () => {
    const {container} = render(
      <VariantSelect resource={createResource()} selectedVariant={undefined} onVariantChange={vi.fn()} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it('should return null when variants array is empty', () => {
    const {container} = render(
      <VariantSelect resource={createResource([])} selectedVariant={undefined} onVariantChange={vi.fn()} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it('should render a select with variant options', () => {
    render(
      <VariantSelect
        resource={createResource(mockVariants)}
        selectedVariant={mockVariants[0]}
        onVariantChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Variant')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('should display the selected variant value', () => {
    render(
      <VariantSelect
        resource={createResource(mockVariants)}
        selectedVariant={mockVariants[1]}
        onVariantChange={vi.fn()}
      />,
    );

    expect(screen.getByRole('combobox')).toHaveTextContent('SECONDARY');
  });

  it('should highlight the variant selector when validation reports a missing variant', () => {
    const notification = new Notification('notification-1', 'Error', 'error');
    notification.addResourceFieldNotification('resource-1_variant', 'Variant is required');

    render(
      <VariantSelect resource={createResource(mockVariants)} selectedVariant={undefined} onVariantChange={vi.fn()} />,
      {wrapper: createWrapper({...defaultContextValue, selectedNotification: notification})},
    );

    expect(screen.getByText('Variant is required')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toHaveAttribute('aria-invalid', 'true');
  });

  it('should call onVariantChange when a variant is selected', () => {
    const mockOnVariantChange = vi.fn();

    render(
      <VariantSelect
        resource={createResource(mockVariants)}
        selectedVariant={mockVariants[0]}
        onVariantChange={mockOnVariantChange}
      />,
    );

    // Open the select
    fireEvent.mouseDown(screen.getByRole('combobox'));
    // Click SECONDARY option
    fireEvent.click(screen.getByText('SECONDARY'));

    expect(mockOnVariantChange).toHaveBeenCalledWith('SECONDARY');
  });
});
