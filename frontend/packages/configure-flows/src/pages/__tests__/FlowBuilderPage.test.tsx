// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import FlowBuilderPage from '../FlowBuilderPage';

// Mock ReactFlowProvider
vi.mock('@xyflow/react', () => ({
  ReactFlowProvider: ({children}: {children: React.ReactNode}) => (
    <div data-testid="react-flow-provider">{children}</div>
  ),
}));

// Mock FlowBuilder
vi.mock('../../components/FlowBuilder', () => ({
  default: () => <div data-testid="flow-builder">Flow Builder</div>,
}));

// Mock FlowBuilderProvider
vi.mock('../../context/FlowBuilderProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => <div data-testid="flow-builder-provider">{children}</div>,
}));

describe('FlowBuilderPage', () => {
  describe('Rendering', () => {
    it('should render the FlowBuilderProvider', () => {
      render(<FlowBuilderPage />);

      expect(screen.getByTestId('flow-builder-provider')).toBeInTheDocument();
    });

    it('should render the ReactFlowProvider inside FlowBuilderProvider', () => {
      render(<FlowBuilderPage />);

      const provider = screen.getByTestId('flow-builder-provider');
      const reactFlowProvider = screen.getByTestId('react-flow-provider');

      expect(provider).toContainElement(reactFlowProvider);
    });

    it('should render the FlowBuilder inside ReactFlowProvider', () => {
      render(<FlowBuilderPage />);

      const reactFlowProvider = screen.getByTestId('react-flow-provider');
      const flowBuilder = screen.getByTestId('flow-builder');

      expect(reactFlowProvider).toContainElement(flowBuilder);
    });

    it('should render components in correct nesting order', () => {
      render(<FlowBuilderPage />);

      const provider = screen.getByTestId('flow-builder-provider');
      const reactFlowProvider = screen.getByTestId('react-flow-provider');
      const flowBuilder = screen.getByTestId('flow-builder');

      // Verify proper nesting: FlowBuilderProvider > ReactFlowProvider > FlowBuilder
      expect(provider).toContainElement(reactFlowProvider);
      expect(reactFlowProvider).toContainElement(flowBuilder);
    });
  });

  describe('Component Integration', () => {
    it('should render FlowBuilder content', () => {
      render(<FlowBuilderPage />);

      expect(screen.getByText('Flow Builder')).toBeInTheDocument();
    });
  });

  describe('Page Structure', () => {
    it('should have all required provider wrappers', () => {
      render(<FlowBuilderPage />);

      expect(screen.getByTestId('flow-builder-provider')).toBeInTheDocument();
      expect(screen.getByTestId('react-flow-provider')).toBeInTheDocument();
      expect(screen.getByTestId('flow-builder')).toBeInTheDocument();
    });

    it('should render without crashing', () => {
      expect(() => render(<FlowBuilderPage />)).not.toThrow();
    });
  });
});
