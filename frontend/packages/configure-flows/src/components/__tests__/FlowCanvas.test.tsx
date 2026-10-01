// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Element} from '../../models/elements';
import type {Step} from '../../models/steps';
import FlowCanvas from '../FlowCanvas';

// Mock @xyflow/react
vi.mock('@xyflow/react', () => ({
  ReactFlowProvider: ({children}: {children: React.ReactNode}) => (
    <div data-testid="react-flow-provider">{children}</div>
  ),
}));

// Mock DecoratedVisualFlow
vi.mock('../visual-flow/DecoratedVisualFlow', () => ({
  default: (props: Record<string, unknown>) => (
    <div data-testid="decorated-visual-flow" data-props={JSON.stringify(Object.keys(props))}>
      Decorated Visual Flow
    </div>
  ),
}));

describe('FlowCanvas', () => {
  const mockResources = {
    steps: [],
    executors: [],
    templates: [],
    widgets: [],
    elements: [],
  };

  const mockProps = {
    resources: mockResources,
    flowTitle: 'Test Flow',
    flowHandle: 'test-flow',
    onFlowTitleChange: vi.fn(),
    mutateComponents: vi.fn((components: Element[]) => components),
    onTemplateLoad: vi.fn(() => [[], [], undefined, undefined] as [never[], never[], undefined, undefined]),
    onWidgetLoad: vi.fn(() => [[], [], null, null] as [never[], never[], null, null]),
    onStepLoad: vi.fn((step: Step) => step),
    nodes: [],
    edges: [],
    onResourceAdd: vi.fn(),
    setNodes: vi.fn(),
    setEdges: vi.fn(),
    onNodesChange: vi.fn(),
    onEdgesChange: vi.fn(),
  };

  it('should render ReactFlowProvider wrapper', () => {
    render(<FlowCanvas {...mockProps} />);

    expect(screen.getByTestId('react-flow-provider')).toBeInTheDocument();
  });

  it('should render DecoratedVisualFlow inside provider', () => {
    render(<FlowCanvas {...mockProps} />);

    expect(screen.getByTestId('decorated-visual-flow')).toBeInTheDocument();
  });

  it('should pass props to DecoratedVisualFlow', () => {
    render(<FlowCanvas {...mockProps} />);

    const decoratedFlow = screen.getByTestId('decorated-visual-flow');
    const propsKeys = JSON.parse(decoratedFlow.getAttribute('data-props') ?? '[]') as string[];

    expect(propsKeys).toContain('resources');
    expect(propsKeys).toContain('flowTitle');
    expect(propsKeys).toContain('flowHandle');
    expect(propsKeys).toContain('onFlowTitleChange');
  });

  it('should render with minimal required props', () => {
    render(<FlowCanvas {...mockProps} />);

    expect(screen.getByText('Decorated Visual Flow')).toBeInTheDocument();
  });

  it('should render with additional optional props', () => {
    const extendedProps = {
      ...mockProps,
      initialNodes: [{id: 'node-1', position: {x: 0, y: 0}, data: {}}],
      initialEdges: [{id: 'edge-1', source: 'node-1', target: 'node-2'}],
      triggerAutoLayoutOnLoad: true,
      onSave: vi.fn(),
    };

    render(<FlowCanvas {...extendedProps} />);

    expect(screen.getByTestId('react-flow-provider')).toBeInTheDocument();
    expect(screen.getByTestId('decorated-visual-flow')).toBeInTheDocument();
  });
});
