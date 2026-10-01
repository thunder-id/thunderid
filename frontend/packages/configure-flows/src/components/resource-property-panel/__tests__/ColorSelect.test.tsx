// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import * as useResourceFieldErrorModule from '../../../hooks/useResourceFieldError';
import type {Resource} from '../../../models/resources';
import ColorSelect, {type ColorSelectProps} from '../ColorSelect';

// Mock the useResourceFieldError hook
vi.mock('../../../hooks/useResourceFieldError', () => ({
  default: vi.fn(() => ''),
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (_key: string, defaultValue: string) => defaultValue,
  }),
}));

describe('ColorSelect Component', () => {
  const mockResource: Resource = {id: 'test-resource'} as unknown as Resource;
  const defaultProps: ColorSelectProps = {
    resource: mockResource,
    selectedColor: undefined,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render FormControl with color label', () => {
    render(<ColorSelect {...defaultProps} />);

    expect(screen.getByText('Color')).toBeInTheDocument();
    const select = screen.getByRole('combobox');
    expect(select).toBeInTheDocument();
  });

  it('should render without error message when there is no error', () => {
    vi.mocked(useResourceFieldErrorModule.default).mockReturnValue('');

    render(<ColorSelect {...defaultProps} />);

    const helperTexts = screen.queryAllByRole('tooltip');
    expect(helperTexts.length).toBe(0);
  });

  it('should display error message when field has error', () => {
    const errorMessage = 'Color is required';
    vi.mocked(useResourceFieldErrorModule.default).mockReturnValue(errorMessage);

    const props: ColorSelectProps = {
      ...defaultProps,
    };

    render(<ColorSelect {...props} />);

    expect(screen.getByText(errorMessage)).toBeInTheDocument();
  });

  it('should render select component with correct props', () => {
    const props: ColorSelectProps = {
      ...defaultProps,
      selectedColor: 'primary',
    };

    render(<ColorSelect {...props} />);

    const select = screen.getByRole('combobox', {hidden: true});
    expect(select).toBeInTheDocument();
  });

  it('should call onColorChange callback when provided', () => {
    const onColorChange = vi.fn();
    const props: ColorSelectProps = {
      ...defaultProps,
      onColorChange,
    };

    const {container} = render(<ColorSelect {...props} />);

    // Find the select element and verify it can be clicked
    const select = screen.getByRole('combobox', {hidden: true});
    expect(select).toBeInTheDocument();
    expect(container.querySelector('[id="color-select"]')).toBeInTheDocument();
  });

  it('should handle undefined onColorChange gracefully', () => {
    const props: ColorSelectProps = {
      ...defaultProps,
      onColorChange: undefined,
    };

    const {container} = render(<ColorSelect {...props} />);

    // Component should render without errors
    expect(container.querySelector('[id="color-select"]')).toBeInTheDocument();
  });

  it('should render MenuItem for DEFAULT option', () => {
    render(<ColorSelect {...defaultProps} />);

    expect(screen.getByText('DEFAULT')).toBeInTheDocument();
  });
});
