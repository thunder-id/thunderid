/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
    flowNodeTypes: {},
    flowEdgeTypes: {},
    setFlowNodeTypes: vi.fn(),
    setFlowEdgeTypes: vi.fn(),
    flowNodes: [],
    setFlowNodes: vi.fn(),
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
});
