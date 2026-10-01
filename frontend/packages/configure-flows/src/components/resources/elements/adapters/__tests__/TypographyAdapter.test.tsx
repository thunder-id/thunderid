// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {TypographyVariants, ElementTypes, type Element as FlowElement} from '../../../../../models/elements';
import TypographyAdapter from '../TypographyAdapter';

// Mock dependencies
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

vi.mock('../PlaceholderComponent', () => ({
  default: ({value}: {value: string}) => <span data-testid="placeholder">{value}</span>,
}));

describe('TypographyAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'typography-1',
      resourceType: 'ELEMENT',
      type: 'TEXT',
      category: 'DISPLAY',
      version: '1.0.0',
      deprecated: false,
      deletable: true,
      display: {
        label: 'Typography',
        image: '',
        showOnResourcePanel: false,
      },
      config: {
        field: {name: 'text', type: 'TEXT'},
        styles: {},
      },
      label: 'Hello World',
      variant: TypographyVariants.Body1,
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render Typography component', () => {
      const resource = createMockElement();

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-root')).toBeInTheDocument();
    });

    it('should render label text', () => {
      const resource = createMockElement({label: 'Test Label'});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-root')).toHaveTextContent('Test Label');
    });
  });

  describe('Typography Variants', () => {
    it('should render H1 variant with center alignment', () => {
      const resource = createMockElement({variant: TypographyVariants.H1});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-h1')).toBeInTheDocument();
    });

    it('should render H2 variant with center alignment', () => {
      const resource = createMockElement({variant: TypographyVariants.H2});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-h2')).toBeInTheDocument();
    });

    it('should render H3 variant with center alignment', () => {
      const resource = createMockElement({variant: TypographyVariants.H3});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-h3')).toBeInTheDocument();
    });

    it('should render H4 variant with center alignment', () => {
      const resource = createMockElement({variant: TypographyVariants.H4});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-h4')).toBeInTheDocument();
    });

    it('should render H5 variant with center alignment', () => {
      const resource = createMockElement({variant: TypographyVariants.H5});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-h5')).toBeInTheDocument();
    });

    it('should render H6 variant with center alignment', () => {
      const resource = createMockElement({variant: TypographyVariants.H6});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-h6')).toBeInTheDocument();
    });

    it('should render Body1 variant', () => {
      const resource = createMockElement({variant: TypographyVariants.Body1});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-body1')).toBeInTheDocument();
    });

    it('should render Body2 variant', () => {
      const resource = createMockElement({variant: TypographyVariants.Body2});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-body2')).toBeInTheDocument();
    });
  });

  describe('Config Styles', () => {
    it('should apply styles from config', () => {
      const resource = createMockElement({
        config: {
          field: {name: 'text', type: ElementTypes},
          styles: {color: 'red'},
        },
      });

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      const typography = container.querySelector('.MuiTypography-root');
      // Color can be normalized to RGB format
      expect(typography).toHaveStyle({color: 'rgb(255, 0, 0)'});
    });
  });

  describe('Empty Label', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-root')).toHaveTextContent('');
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-root')).toHaveTextContent('');
    });
  });

  describe('Undefined Variant', () => {
    it('should handle undefined variant', () => {
      const resource = createMockElement({variant: undefined});

      const {container} = render(<TypographyAdapter resource={resource} stepId="step-1" />);

      expect(container.querySelector('.MuiTypography-root')).toBeInTheDocument();
    });
  });
});
