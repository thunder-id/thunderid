// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment */

import {render, screen, fireEvent} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowConfigContext, {type FlowConfigContextProps} from '../../../context/FlowConfigContext';
import {EdgeStyleTypes} from '../../../models/steps';
import CanvasToolbar from '../CanvasToolbar';

const mockFitView = vi.fn().mockResolvedValue(true);
const mockZoomIn = vi.fn().mockResolvedValue(true);
const mockZoomOut = vi.fn().mockResolvedValue(true);

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    fitView: mockFitView,
    zoomIn: mockZoomIn,
    zoomOut: mockZoomOut,
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

vi.mock('../EdgeStyleSelector', () => ({
  default: ({anchorEl, onClose}: any) => (
    <div data-testid="edge-style-menu" data-open={Boolean(anchorEl)}>
      <button data-testid="close-menu" type="button" onClick={onClose}>
        Close
      </button>
    </div>
  ),
}));

vi.mock('../../../utils/getEdgeStyleIcon', () => ({
  default: () => <span data-testid="edge-style-icon" />,
}));

describe('CanvasToolbar', () => {
  const mockOnAutoLayout = vi.fn();

  const defaultFlowConfigValue: FlowConfigContextProps = {
    ElementFactory: () => null,
    ResourceProperties: () => null,
    flowCompletionConfigs: {},
    setFlowCompletionConfigs: vi.fn(),
    isVerboseMode: false,
    setIsVerboseMode: vi.fn(),
    edgeStyle: EdgeStyleTypes.SmoothStep,
    setEdgeStyle: vi.fn(),
    isSnapToGridEnabled: false,
    setIsSnapToGridEnabled: vi.fn(),
    isMiniMapVisible: true,
    setIsMiniMapVisible: vi.fn(),
    flowNodeTypes: {},
    flowEdgeTypes: {},
    setFlowNodeTypes: vi.fn(),
    setFlowEdgeTypes: vi.fn(),
    flowNodes: [],
    setFlowNodes: vi.fn(),
    flowEdges: [],
    setFlowEdges: vi.fn(),
    graphValidationRules: [],
    setGraphValidationRules: vi.fn(),
  };

  function Wrapper({children}: {children: ReactNode}) {
    return <FlowConfigContext.Provider value={defaultFlowConfigValue}>{children}</FlowConfigContext.Provider>;
  }

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the toolbar with correct role', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.getByRole('toolbar')).toBeInTheDocument();
  });

  it('should render auto-layout button', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.getByLabelText('flows:core.headerPanel.autoLayout')).toBeInTheDocument();
  });

  it('should call onAutoLayout when auto-layout button is clicked', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    fireEvent.click(screen.getByLabelText('flows:core.headerPanel.autoLayout'));

    expect(mockOnAutoLayout).toHaveBeenCalledTimes(1);
  });

  it('should render edge style button with menu trigger attributes', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    const edgeButton = screen.getByLabelText('flows:core.headerPanel.edgeStyleTooltip');
    expect(edgeButton).toHaveAttribute('aria-haspopup', 'true');
    expect(edgeButton).toHaveAttribute('aria-expanded', 'false');
  });

  it('should render edge style icon', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.getByTestId('edge-style-icon')).toBeInTheDocument();
  });

  it('should render edge style menu', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.getByTestId('edge-style-menu')).toBeInTheDocument();
  });

  it('should render zoom out button', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.getByLabelText('Zoom out')).toBeInTheDocument();
  });

  it('should call zoomOut when zoom out button is clicked', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    fireEvent.click(screen.getByLabelText('Zoom out'));

    expect(mockZoomOut).toHaveBeenCalledTimes(1);
  });

  it('should render zoom in button', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.getByLabelText('Zoom in')).toBeInTheDocument();
  });

  it('should call zoomIn when zoom in button is clicked', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    fireEvent.click(screen.getByLabelText('Zoom in'));

    expect(mockZoomIn).toHaveBeenCalledTimes(1);
  });

  it('should render fit view button', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.getByLabelText('Fit view')).toBeInTheDocument();
  });

  it('should call fitView when fit view button is clicked', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    fireEvent.click(screen.getByLabelText('Fit view'));

    expect(mockFitView).toHaveBeenCalledWith({padding: 0.2, duration: 300});
  });

  it('should not render undo/redo controls when no history handlers are provided', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} />, {wrapper: Wrapper});

    expect(screen.queryByLabelText('Undo')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Redo')).not.toBeInTheDocument();
  });

  it('should render undo/redo controls when history handlers are provided', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} onUndo={vi.fn()} onRedo={vi.fn()} canUndo canRedo />, {
      wrapper: Wrapper,
    });

    expect(screen.getByRole('button', {name: 'Undo'})).toBeEnabled();
    expect(screen.getByRole('button', {name: 'Redo'})).toBeEnabled();
  });

  it('should disable undo/redo when there is nothing to undo/redo', () => {
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} onUndo={vi.fn()} onRedo={vi.fn()} />, {wrapper: Wrapper});

    expect(screen.getByRole('button', {name: 'Undo'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Redo'})).toBeDisabled();
  });

  it('should call onUndo and onRedo when the buttons are clicked', () => {
    const onUndo = vi.fn();
    const onRedo = vi.fn();
    render(<CanvasToolbar onAutoLayout={mockOnAutoLayout} onUndo={onUndo} onRedo={onRedo} canUndo canRedo />, {
      wrapper: Wrapper,
    });

    fireEvent.click(screen.getByRole('button', {name: 'Undo'}));
    fireEvent.click(screen.getByRole('button', {name: 'Redo'}));

    expect(onUndo).toHaveBeenCalledTimes(1);
    expect(onRedo).toHaveBeenCalledTimes(1);
  });

  describe('Verbose mode toggle', () => {
    const renderWithConfig = (overrides: Partial<FlowConfigContextProps>) =>
      render(
        <FlowConfigContext.Provider value={{...defaultFlowConfigValue, ...overrides}}>
          <CanvasToolbar onAutoLayout={mockOnAutoLayout} />
        </FlowConfigContext.Provider>,
      );

    it('should report the compact view as unpressed while in detailed (verbose) mode', () => {
      renderWithConfig({isVerboseMode: true});

      const toggle = screen.getByRole('button', {name: 'Compact view'});
      expect(toggle).toHaveAttribute('aria-pressed', 'false');
    });

    it('should report the compact view as pressed while in compact mode', () => {
      renderWithConfig({isVerboseMode: false});

      // The name stays put so the state is not announced twice, and inverted.
      const toggle = screen.getByRole('button', {name: 'Compact view'});
      expect(toggle).toHaveAttribute('aria-pressed', 'true');
    });

    it('should flip the verbose mode when the toggle is clicked', () => {
      const setIsVerboseMode = vi.fn();
      renderWithConfig({isVerboseMode: true, setIsVerboseMode});

      fireEvent.click(screen.getByRole('button', {name: 'Compact view'}));

      expect(setIsVerboseMode).toHaveBeenCalledTimes(1);
      const updater = setIsVerboseMode.mock.calls[0][0] as (prev: boolean) => boolean;
      expect(updater(true)).toBe(false);
      expect(updater(false)).toBe(true);
    });
  });

  describe('Snap to grid toggle', () => {
    const renderWithConfig = (overrides: Partial<FlowConfigContextProps>) =>
      render(
        <FlowConfigContext.Provider value={{...defaultFlowConfigValue, ...overrides}}>
          <CanvasToolbar onAutoLayout={mockOnAutoLayout} />
        </FlowConfigContext.Provider>,
      );

    it('should report the snap-to-grid state via aria-pressed', () => {
      renderWithConfig({isSnapToGridEnabled: false});

      expect(screen.getByRole('button', {name: 'Snap to grid'})).toHaveAttribute('aria-pressed', 'false');
    });

    it('should flip the snap-to-grid state when the toggle is clicked', () => {
      const setIsSnapToGridEnabled = vi.fn();
      renderWithConfig({isSnapToGridEnabled: false, setIsSnapToGridEnabled});

      fireEvent.click(screen.getByRole('button', {name: 'Snap to grid'}));

      expect(setIsSnapToGridEnabled).toHaveBeenCalledTimes(1);
      const updater = setIsSnapToGridEnabled.mock.calls[0][0] as (prev: boolean) => boolean;
      expect(updater(false)).toBe(true);
      expect(updater(true)).toBe(false);
    });
  });

  describe('Minimap toggle', () => {
    const renderWithConfig = (overrides: Partial<FlowConfigContextProps>) =>
      render(
        <FlowConfigContext.Provider value={{...defaultFlowConfigValue, ...overrides}}>
          <CanvasToolbar onAutoLayout={mockOnAutoLayout} />
        </FlowConfigContext.Provider>,
      );

    it('should report the minimap visibility via aria-pressed', () => {
      renderWithConfig({isMiniMapVisible: true});

      expect(screen.getByRole('button', {name: 'Minimap'})).toHaveAttribute('aria-pressed', 'true');
    });

    it('should flip the minimap visibility when the toggle is clicked', () => {
      const setIsMiniMapVisible = vi.fn();
      renderWithConfig({isMiniMapVisible: true, setIsMiniMapVisible});

      fireEvent.click(screen.getByRole('button', {name: 'Minimap'}));

      expect(setIsMiniMapVisible).toHaveBeenCalledTimes(1);
      const updater = setIsMiniMapVisible.mock.calls[0][0] as (prev: boolean) => boolean;
      expect(updater(true)).toBe(false);
      expect(updater(false)).toBe(true);
    });
  });
});
