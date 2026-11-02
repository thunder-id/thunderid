// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {DividerVariants, type Element as FlowElement} from '../../../../../models/elements';
import DividerAdapter from '../DividerAdapter';

// Mock dependencies
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

describe('DividerAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'divider-1',
      type: 'DIVIDER',
      category: 'DISPLAY',
      config: {},
      variant: DividerVariants.Horizontal,
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render a Divider component', () => {
      const resource = createMockElement();

      const {container} = render(<DividerAdapter resource={resource} />);

      expect(container.querySelector('hr')).toBeInTheDocument();
    });

    it('should render label text when provided', () => {
      const resource = createMockElement({label: 'OR'});

      render(<DividerAdapter resource={resource} />);

      expect(screen.getByText('OR')).toBeInTheDocument();
    });

    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<DividerAdapter resource={resource} />);

      expect(container.querySelector('hr')).toBeInTheDocument();
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<DividerAdapter resource={resource} />);

      expect(container.querySelector('hr')).toBeInTheDocument();
    });
  });

  describe('Divider Variants', () => {
    it('should render horizontal divider', () => {
      const resource = createMockElement({variant: DividerVariants.Horizontal});

      const {container} = render(<DividerAdapter resource={resource} />);

      const divider = container.querySelector('hr');
      expect(divider).toBeInTheDocument();
    });

    it('should render vertical divider', () => {
      const resource = createMockElement({variant: DividerVariants.Vertical});

      const {container} = render(<DividerAdapter resource={resource} />);

      // Vertical dividers in MUI may render as div instead of hr
      const divider = container.querySelector('.MuiDivider-root');
      expect(divider).toBeInTheDocument();
    });

    it('should handle other variant values as MUI variant', () => {
      const resource = createMockElement({variant: 'fullWidth' as typeof DividerVariants.Horizontal});

      const {container} = render(<DividerAdapter resource={resource} />);

      const divider = container.querySelector('hr');
      expect(divider).toBeInTheDocument();
    });

    it('should handle undefined variant', () => {
      const resource = createMockElement({variant: undefined});

      const {container} = render(<DividerAdapter resource={resource} />);

      const divider = container.querySelector('hr');
      expect(divider).toBeInTheDocument();
    });
  });

  describe('Different Resource IDs', () => {
    it('should render with different resource IDs', () => {
      const resource1 = createMockElement({id: 'divider-1', label: 'First'});
      const resource2 = createMockElement({id: 'divider-2', label: 'Second'});

      render(<DividerAdapter resource={resource1} />);
      render(<DividerAdapter resource={resource2} />);

      expect(screen.getByText('First')).toBeInTheDocument();
      expect(screen.getByText('Second')).toBeInTheDocument();
    });
  });
});
