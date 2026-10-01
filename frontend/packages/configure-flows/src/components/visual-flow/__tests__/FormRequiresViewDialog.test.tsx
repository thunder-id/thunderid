// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FormRequiresViewDialog, {type DropScenario} from '../FormRequiresViewDialog';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        // Form on canvas
        'flows:core.dialogs.formRequiresView.formOnCanvas.title': 'Form requires a View',
        'flows:core.dialogs.formRequiresView.formOnCanvas.description': 'A Form component must be inside a View step.',
        'flows:core.dialogs.formRequiresView.formOnCanvas.alertMessage':
          'A View step will be created to contain this Form.',
        'flows:core.dialogs.formRequiresView.formOnCanvas.confirmButton': 'Create View',
        // Input on canvas
        'flows:core.dialogs.formRequiresView.inputOnCanvas.title': 'Input requires a View and Form',
        'flows:core.dialogs.formRequiresView.inputOnCanvas.description':
          'An Input component must be inside a Form, which is inside a View step.',
        'flows:core.dialogs.formRequiresView.inputOnCanvas.alertMessage':
          'A View step with a Form will be created to contain this Input.',
        'flows:core.dialogs.formRequiresView.inputOnCanvas.confirmButton': 'Create View and Form',
        // Input on view
        'flows:core.dialogs.formRequiresView.inputOnView.title': 'Input requires a Form',
        'flows:core.dialogs.formRequiresView.inputOnView.description': 'An Input component must be inside a Form.',
        'flows:core.dialogs.formRequiresView.inputOnView.alertMessage': 'A Form will be created to contain this Input.',
        'flows:core.dialogs.formRequiresView.inputOnView.confirmButton': 'Create Form',
        // Widget on canvas
        'flows:core.dialogs.formRequiresView.widgetOnCanvas.title': 'Widget requires a View',
        'flows:core.dialogs.formRequiresView.widgetOnCanvas.description':
          'A Widget component must be inside a View step.',
        'flows:core.dialogs.formRequiresView.widgetOnCanvas.alertMessage':
          'A View step will be created to contain this Widget.',
        'flows:core.dialogs.formRequiresView.widgetOnCanvas.confirmButton': 'Create View',
        // Common
        'flows:core.dialogs.formRequiresView.cancelButton': 'Cancel',
      };
      return translations[key] || key;
    },
  }),
}));

describe('FormRequiresViewDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnConfirm = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Dialog Visibility', () => {
    it('should render dialog when open is true', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should not render dialog content when open is false', () => {
      render(
        <FormRequiresViewDialog
          open={false}
          scenario="form-on-canvas"
          onClose={mockOnClose}
          onConfirm={mockOnConfirm}
        />,
      );

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  describe('Form on Canvas Scenario', () => {
    it('should display correct title for form-on-canvas', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByText('Form requires a View')).toBeInTheDocument();
    });

    it('should display correct description for form-on-canvas', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByText('A Form component must be inside a View step.')).toBeInTheDocument();
    });

    it('should display correct alert message for form-on-canvas', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByText('A View step will be created to contain this Form.')).toBeInTheDocument();
    });

    it('should display correct confirm button text for form-on-canvas', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByRole('button', {name: 'Create View'})).toBeInTheDocument();
    });
  });

  describe('Input on Canvas Scenario', () => {
    it('should display correct title for input-on-canvas', () => {
      render(
        <FormRequiresViewDialog open scenario="input-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />,
      );

      expect(screen.getByText('Input requires a View and Form')).toBeInTheDocument();
    });

    it('should display correct description for input-on-canvas', () => {
      render(
        <FormRequiresViewDialog open scenario="input-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />,
      );

      expect(
        screen.getByText('An Input component must be inside a Form, which is inside a View step.'),
      ).toBeInTheDocument();
    });

    it('should display correct confirm button text for input-on-canvas', () => {
      render(
        <FormRequiresViewDialog open scenario="input-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />,
      );

      expect(screen.getByRole('button', {name: 'Create View and Form'})).toBeInTheDocument();
    });
  });

  describe('Input on View Scenario', () => {
    it('should display correct title for input-on-view', () => {
      render(<FormRequiresViewDialog open scenario="input-on-view" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByText('Input requires a Form')).toBeInTheDocument();
    });

    it('should display correct description for input-on-view', () => {
      render(<FormRequiresViewDialog open scenario="input-on-view" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByText('An Input component must be inside a Form.')).toBeInTheDocument();
    });

    it('should display correct confirm button text for input-on-view', () => {
      render(<FormRequiresViewDialog open scenario="input-on-view" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByRole('button', {name: 'Create Form'})).toBeInTheDocument();
    });
  });

  describe('Widget on Canvas Scenario', () => {
    it('should display correct title for widget-on-canvas', () => {
      render(
        <FormRequiresViewDialog open scenario="widget-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />,
      );

      expect(screen.getByText('Widget requires a View')).toBeInTheDocument();
    });

    it('should display correct description for widget-on-canvas', () => {
      render(
        <FormRequiresViewDialog open scenario="widget-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />,
      );

      expect(screen.getByText('A Widget component must be inside a View step.')).toBeInTheDocument();
    });

    it('should display correct confirm button text for widget-on-canvas', () => {
      render(
        <FormRequiresViewDialog open scenario="widget-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />,
      );

      expect(screen.getByRole('button', {name: 'Create View'})).toBeInTheDocument();
    });
  });

  describe('Button Actions', () => {
    it('should call onClose when Cancel button is clicked', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      fireEvent.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should call onConfirm when confirm button is clicked', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      const confirmButton = screen.getByRole('button', {name: 'Create View'});
      fireEvent.click(confirmButton);

      expect(mockOnConfirm).toHaveBeenCalledTimes(1);
    });

    it('should display Cancel button for all scenarios', () => {
      const scenarios: DropScenario[] = ['form-on-canvas', 'input-on-canvas', 'input-on-view', 'widget-on-canvas'];

      scenarios.forEach((scenario) => {
        const {unmount} = render(
          <FormRequiresViewDialog open scenario={scenario} onClose={mockOnClose} onConfirm={mockOnConfirm} />,
        );

        expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
        unmount();
      });
    });
  });

  describe('Alert Component', () => {
    it('should render an info alert for all scenarios', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();
    });

    it('should display alert with info severity', () => {
      render(
        <FormRequiresViewDialog open scenario="input-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />,
      );

      const alert = screen.getByRole('alert');
      expect(alert).toHaveClass('MuiAlert-standardInfo');
    });
  });

  describe('Dialog Structure', () => {
    it('should render dialog title', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      expect(screen.getByRole('heading')).toBeInTheDocument();
    });

    it('should render two buttons (Cancel and Confirm)', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      const buttons = screen.getAllByRole('button');
      expect(buttons).toHaveLength(2);
    });

    it('should render confirm button with contained variant', () => {
      render(<FormRequiresViewDialog open scenario="form-on-canvas" onClose={mockOnClose} onConfirm={mockOnConfirm} />);

      const confirmButton = screen.getByRole('button', {name: 'Create View'});
      expect(confirmButton).toHaveClass('MuiButton-contained');
    });
  });

  describe('Scenario Coverage', () => {
    it('should handle all four drop scenarios correctly', () => {
      const scenarios: DropScenario[] = ['form-on-canvas', 'input-on-canvas', 'input-on-view', 'widget-on-canvas'];

      scenarios.forEach((scenario) => {
        const {unmount} = render(
          <FormRequiresViewDialog open scenario={scenario} onClose={mockOnClose} onConfirm={mockOnConfirm} />,
        );

        // Each scenario should render without errors
        expect(screen.getByRole('dialog')).toBeInTheDocument();
        expect(screen.getByRole('alert')).toBeInTheDocument();
        expect(screen.getAllByRole('button')).toHaveLength(2);

        unmount();
      });
    });
  });
});
