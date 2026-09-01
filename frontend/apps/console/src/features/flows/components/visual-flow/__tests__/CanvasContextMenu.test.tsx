// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../models/resources';
import CanvasContextMenu, {type CanvasContextMenuTarget} from '../CanvasContextMenu';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

vi.mock('../../ResourceDisplayImage', () => ({
  default: ({label}: {label?: string}) => <span data-testid="resource-display-image" data-label={label} />,
}));

const nodeTarget: CanvasContextMenuTarget = {
  kind: 'node',
  nodeId: 'node-1',
  position: {left: 100, top: 200},
};

const paneTarget: CanvasContextMenuTarget = {
  kind: 'pane',
  position: {left: 50, top: 60},
};

const stepResources = [
  {type: 'VIEW', display: {label: 'Blank View', image: 'View', showOnResourcePanel: true}},
  {type: 'CALL', display: {label: 'Flow', image: 'Workflow', showOnResourcePanel: true}},
] as unknown as Resource[];

describe('CanvasContextMenu', () => {
  const defaultProps = {
    onClose: vi.fn(),
    onDuplicate: vi.fn(),
    onDelete: vi.fn(),
    onOpenProperties: vi.fn(),
    onPreviewFromStep: vi.fn(),
    onAddStep: vi.fn(),
    onAutoLayout: vi.fn(),
    onFitView: vi.fn(),
    addableSteps: stepResources,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render nothing while closed', () => {
    render(<CanvasContextMenu {...defaultProps} target={null} />);

    expect(screen.queryByTestId('canvas-context-menu-duplicate')).not.toBeInTheDocument();
    expect(screen.queryByTestId('canvas-context-menu-add-step')).not.toBeInTheDocument();
  });

  describe('Node menu', () => {
    it('should render the node actions', () => {
      render(<CanvasContextMenu {...defaultProps} target={nodeTarget} canDuplicate canDelete hasProperties />);

      expect(screen.getByTestId('canvas-context-menu-duplicate')).toBeInTheDocument();
      expect(screen.getByTestId('canvas-context-menu-open-properties')).toBeInTheDocument();
      expect(screen.getByTestId('canvas-context-menu-preview')).toBeInTheDocument();
      expect(screen.getByTestId('canvas-context-menu-delete')).toBeInTheDocument();
    });

    it('should hide the properties action for nodes without a properties panel', () => {
      render(<CanvasContextMenu {...defaultProps} target={nodeTarget} canDuplicate canDelete />);

      expect(screen.queryByTestId('canvas-context-menu-open-properties')).not.toBeInTheDocument();
    });

    it('should disable duplicate and delete for protected nodes', () => {
      render(<CanvasContextMenu {...defaultProps} target={nodeTarget} />);

      expect(screen.getByTestId('canvas-context-menu-duplicate')).toHaveAttribute('aria-disabled', 'true');
      expect(screen.getByTestId('canvas-context-menu-delete')).toHaveAttribute('aria-disabled', 'true');
    });

    it('should invoke the action and close on click', () => {
      render(<CanvasContextMenu {...defaultProps} target={nodeTarget} canDuplicate canDelete hasProperties />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-duplicate'));

      expect(defaultProps.onDuplicate).toHaveBeenCalledTimes(1);
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });

    it('should invoke delete and close on click', () => {
      render(<CanvasContextMenu {...defaultProps} target={nodeTarget} canDuplicate canDelete />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-delete'));

      expect(defaultProps.onDelete).toHaveBeenCalledTimes(1);
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });

    it('should invoke preview-from-step and close on click', () => {
      render(<CanvasContextMenu {...defaultProps} target={nodeTarget} canDuplicate canDelete />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-preview'));

      expect(defaultProps.onPreviewFromStep).toHaveBeenCalledTimes(1);
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Pane menu', () => {
    it('should render the canvas actions', () => {
      render(<CanvasContextMenu {...defaultProps} target={paneTarget} />);

      expect(screen.getByTestId('canvas-context-menu-add-step')).toBeInTheDocument();
      expect(screen.getByTestId('canvas-context-menu-auto-layout')).toBeInTheDocument();
      expect(screen.getByTestId('canvas-context-menu-fit-view')).toBeInTheDocument();
    });

    it('should invoke auto-layout and close on click', () => {
      render(<CanvasContextMenu {...defaultProps} target={paneTarget} />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-auto-layout'));

      expect(defaultProps.onAutoLayout).toHaveBeenCalledTimes(1);
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });

    it('should invoke fit view and close on click', () => {
      render(<CanvasContextMenu {...defaultProps} target={paneTarget} />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-fit-view'));

      expect(defaultProps.onFitView).toHaveBeenCalledTimes(1);
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });

    it('should drill into the add-step list and add the chosen step', () => {
      render(<CanvasContextMenu {...defaultProps} target={paneTarget} />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-add-step'));

      expect(screen.getByText('Blank View')).toBeInTheDocument();
      expect(screen.getByText('Flow')).toBeInTheDocument();
      expect(defaultProps.onClose).not.toHaveBeenCalled();

      fireEvent.click(screen.getByTestId('canvas-context-menu-add-step-0'));

      expect(defaultProps.onAddStep).toHaveBeenCalledWith(stepResources[0]);
      expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
    });

    it('should return to the root menu from the add-step list', () => {
      render(<CanvasContextMenu {...defaultProps} target={paneTarget} />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-add-step'));
      fireEvent.click(screen.getByTestId('canvas-context-menu-add-step-back'));

      expect(screen.getByTestId('canvas-context-menu-add-step')).toBeInTheDocument();
      expect(defaultProps.onClose).not.toHaveBeenCalled();
    });

    it('should reset the drill-in when the menu is reopened', () => {
      const {rerender} = render(<CanvasContextMenu {...defaultProps} target={paneTarget} />);

      fireEvent.click(screen.getByTestId('canvas-context-menu-add-step'));
      expect(screen.queryByTestId('canvas-context-menu-add-step')).not.toBeInTheDocument();

      rerender(<CanvasContextMenu {...defaultProps} target={null} />);
      rerender(<CanvasContextMenu {...defaultProps} target={{...paneTarget}} />);

      expect(screen.getByTestId('canvas-context-menu-add-step')).toBeInTheDocument();
    });
  });
});
