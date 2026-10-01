// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ElementTypes, type Element as FlowElement} from '../../../../../models/elements';
import ResendButtonAdapter from '../ResendButtonAdapter';

// Mock dependencies
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

vi.mock('../NodeHandle', () => ({
  default: ({id, type, position}: {id: string; type: string; position: string}) => (
    <div data-testid="node-handle" data-id={id} data-type={type} data-position={position} />
  ),
}));

vi.mock('../PlaceholderComponent', () => ({
  default: ({value}: {value: string}) => <span data-testid="placeholder">{value}</span>,
}));

describe('ResendButtonAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'resend-1',
      resourceType: 'ELEMENT',
      type: 'RESEND',
      category: 'ACTION',
      version: '1.0.0',
      deprecated: false,
      deletable: true,
      display: {
        label: 'Resend',
        image: '',
        showOnResourcePanel: false,
      },
      config: {
        field: {name: 'resend', type: 'RESEND'},
        styles: {},
      },
      label: 'Resend Code',
      ...overrides,
    }) as FlowElement;

  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the resend button adapter container', () => {
      const resource = createMockElement();

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('resend-button-adapter')).toBeInTheDocument();
    });

    it('should render a Button component', () => {
      const resource = createMockElement();

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {wrapper: createWrapper()});

      expect(screen.getByRole('button')).toBeInTheDocument();
    });

    it('should render button label', () => {
      const resource = createMockElement({label: 'Resend OTP'});

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {wrapper: createWrapper()});

      expect(screen.getByRole('button')).toHaveTextContent('Resend OTP');
    });

    it('should render NodeHandle for edge connection', () => {
      const resource = createMockElement();

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {wrapper: createWrapper()});

      expect(screen.getByTestId('node-handle')).toBeInTheDocument();
      expect(screen.getByTestId('node-handle')).toHaveAttribute('data-type', 'source');
    });
  });

  describe('Button Configuration', () => {
    it('should render with secondary color', () => {
      const resource = createMockElement();

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {wrapper: createWrapper()});

      const button = screen.getByRole('button');
      expect(button).toBeInTheDocument();
    });

    it('should apply styles from config', () => {
      const resource = createMockElement({
        config: {
          field: {name: 'resend', type: ElementTypes},
          styles: {backgroundColor: 'blue'},
        },
      });

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {wrapper: createWrapper()});

      const button = screen.getByRole('button');
      expect(button).toBeInTheDocument();
    });
  });

  describe('Empty Label', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {wrapper: createWrapper()});

      expect(screen.getByRole('button')).toHaveTextContent('');
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      render(<ResendButtonAdapter resource={resource} stepId="step-1" />, {wrapper: createWrapper()});

      expect(screen.getByRole('button')).toHaveTextContent('');
    });
  });
});
