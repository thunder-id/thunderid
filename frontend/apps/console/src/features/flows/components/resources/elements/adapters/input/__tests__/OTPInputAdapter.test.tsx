// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {ReactFlowProvider, type Edge, type Node} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import OTPInputAdapter from '../OTPInputAdapter';
import type {Element as FlowElement} from '@/features/flows/models/elements';

// Mock dependencies
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

vi.mock('@/features/flows/components/resources/elements/hint', () => ({
  Hint: ({hint}: {hint: string}) => <span data-testid="hint">{hint}</span>,
}));

vi.mock('@/features/flows/components/resources/elements/adapters/PlaceholderComponent', () => ({
  default: ({value}: {value: string}) => <span data-testid="placeholder">{value}</span>,
}));

describe('OTPInputAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'otp-1',
      type: 'OTP_INPUT',
      category: 'FIELD',
      config: {},
      label: 'Enter OTP',
      inputType: 'text',
      ...overrides,
    }) as FlowElement;

  const renderAdapter = (resource: FlowElement, nodes: Node[] = [], edges: Edge[] = []) =>
    render(
      <ReactFlowProvider initialNodes={nodes} initialEdges={edges}>
        <OTPInputAdapter resource={resource} stepId="prompt" />
      </ReactFlowProvider>,
    );

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render InputLabel component', () => {
      const resource = createMockElement();

      const {container} = renderAdapter(resource);

      expect(container.querySelector('.MuiInputLabel-root')).toBeInTheDocument();
    });

    it('should render label text', () => {
      const resource = createMockElement({label: 'Verification Code'});

      renderAdapter(resource);

      expect(screen.getByText('Verification Code')).toBeInTheDocument();
    });

    it('should render 6 input boxes for OTP', () => {
      const resource = createMockElement();

      const {container} = renderAdapter(resource);

      const inputs = container.querySelectorAll('.MuiOutlinedInput-root');
      expect(inputs).toHaveLength(6);
    });

    it('should render the length configured on the upstream Generate OTP node', () => {
      const resource = createMockElement();
      const nodes: Node[] = [
        {
          id: 'generate',
          position: {x: 0, y: 0},
          data: {
            action: {executor: {name: 'OTPExecutor', mode: 'generate'}},
            properties: {otpLength: 8},
          },
        },
        {id: 'prompt', position: {x: 0, y: 0}, data: {}},
      ];
      const edges: Edge[] = [{id: 'generate-prompt', source: 'generate', target: 'prompt'}];

      const {container} = renderAdapter(resource, nodes, edges);

      expect(container.querySelectorAll('.MuiOutlinedInput-root')).toHaveLength(8);
    });
  });

  describe('Required Field', () => {
    it('should show required indicator when required is true', () => {
      const resource = createMockElement({required: true});

      const {container} = renderAdapter(resource);

      expect(container.querySelector('.MuiFormLabel-asterisk')).toBeInTheDocument();
    });

    it('should not show required indicator when required is false', () => {
      const resource = createMockElement({required: false});

      const {container} = renderAdapter(resource);

      expect(container.querySelector('.MuiFormLabel-asterisk')).not.toBeInTheDocument();
    });
  });

  describe('Hint Text', () => {
    it('should render hint when provided', () => {
      const resource = createMockElement({hint: 'Check your email for the code'});

      renderAdapter(resource);

      expect(screen.getByTestId('hint')).toHaveTextContent('Check your email for the code');
    });

    it('should not render hint when not provided', () => {
      const resource = createMockElement({hint: undefined});

      renderAdapter(resource);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });

    it('should not render hint when empty', () => {
      const resource = createMockElement({hint: ''});

      renderAdapter(resource);

      expect(screen.queryByTestId('hint')).not.toBeInTheDocument();
    });
  });

  describe('Placeholder', () => {
    it('should render placeholder on OTP inputs when provided', () => {
      const resource = createMockElement({placeholder: '0'});

      const {container} = renderAdapter(resource);

      const inputs = container.querySelectorAll('input');
      inputs.forEach((input) => {
        expect(input).toHaveAttribute('placeholder', '0');
      });
    });

    it('should render empty placeholder when not provided', () => {
      const resource = createMockElement({placeholder: undefined});

      const {container} = renderAdapter(resource);

      const inputs = container.querySelectorAll('input');
      inputs.forEach((input) => {
        expect(input).toHaveAttribute('placeholder', '');
      });
    });
  });

  describe('Input Type', () => {
    it('should apply input type to OTP fields', () => {
      const resource = createMockElement({inputType: 'number'});

      const {container} = renderAdapter(resource);

      const inputs = container.querySelectorAll('input');
      inputs.forEach((input) => {
        expect(input).toHaveAttribute('type', 'number');
      });
    });
  });

  describe('Custom Styling', () => {
    it('should apply className when provided', () => {
      const resource = createMockElement({classes: 'custom-otp'});

      const {container} = renderAdapter(resource);

      expect(container.firstChild).toHaveClass('custom-otp');
    });

    it('should apply styles to inputs when provided', () => {
      const resource = createMockElement({styles: {width: '40px'}});

      const {container} = renderAdapter(resource);

      const outlinedInputs = container.querySelectorAll('.MuiOutlinedInput-root');
      outlinedInputs.forEach((input) => {
        expect(input).toHaveStyle({width: '40px'});
      });
    });
  });

  describe('Empty Label', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = renderAdapter(resource);

      const label = container.querySelector('.MuiInputLabel-root');
      expect(label).toHaveTextContent('');
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = renderAdapter(resource);

      const label = container.querySelector('.MuiInputLabel-root');
      expect(label).toHaveTextContent('');
    });
  });
});
