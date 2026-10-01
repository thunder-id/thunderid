// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ConfigureFlowName from '../ConfigureFlowName';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (_key: string, defaultValue: string) => defaultValue,
  }),
}));

// Mock generateRandomHumanReadableIdentifiers
const mockSuggestions = ['Cosmic Gateway', 'Lunar Portal', 'Solar Bridge'];

vi.mock('@thunderid/utils', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/utils')>()),
  generateRandomHumanReadableIdentifiers: () => mockSuggestions,
}));

// Mock useTheme
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual('@wso2/oxygen-ui');
  return {
    ...actual,
    useTheme: () => ({
      vars: {palette: {warning: {main: '#ff9800'}}},
    }),
  };
});

// Mock oxygen-ui-icons-react
vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Lightbulb: ({size}: {size: number}) => <span data-testid="icon-lightbulb">{size}</span>,
  };
});

describe('ConfigureFlowName', () => {
  const mockOnChange = vi.fn();
  const mockOnReadyChange = vi.fn();

  const defaultProps = {
    value: {name: '', handle: ''},
    onChange: mockOnChange,
    onReadyChange: mockOnReadyChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the component with data-testid', () => {
      render(<ConfigureFlowName {...defaultProps} />);

      expect(screen.getByTestId('configure-flow-name')).toBeInTheDocument();
    });

    it('should render the title', () => {
      render(<ConfigureFlowName {...defaultProps} />);

      expect(screen.getByText("Let's collect some details about your flow")).toBeInTheDocument();
    });

    it('should render the name input with label', () => {
      render(<ConfigureFlowName {...defaultProps} />);

      expect(screen.getByText('Flow name')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('e.g. Customer Sign-in')).toBeInTheDocument();
    });

    it('should render the handle input with label', () => {
      render(<ConfigureFlowName {...defaultProps} />);

      expect(screen.getByText('Handle')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('e.g. customer-sign-in')).toBeInTheDocument();
    });

    it('should render a name suggestion', () => {
      render(<ConfigureFlowName {...defaultProps} />);

      expect(screen.getByText(mockSuggestions[0])).toBeInTheDocument();
    });
  });

  describe('Name Input', () => {
    it('should call onChange with name and auto-derived handle when name changes', async () => {
      render(<ConfigureFlowName {...defaultProps} />);

      const nameInput = screen.getByPlaceholderText('e.g. Customer Sign-in');
      fireEvent.change(nameInput, {target: {value: 'My Flow'}});

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith({name: 'My Flow', handle: 'my-flow'});
      });
    });

    it('should sanitize handle by removing non-alphanumeric characters', async () => {
      render(<ConfigureFlowName {...defaultProps} />);

      const nameInput = screen.getByPlaceholderText('e.g. Customer Sign-in');
      fireEvent.change(nameInput, {target: {value: 'My Flow @#$!'}});

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith({name: 'My Flow @#$!', handle: 'my-flow-'});
      });
    });
  });

  describe('Handle Input', () => {
    it('should stop auto-derivation when handle is manually edited', async () => {
      render(<ConfigureFlowName {...defaultProps} />);

      const handleInput = screen.getByPlaceholderText('e.g. customer-sign-in');
      fireEvent.change(handleInput, {target: {value: 'custom-handle'}});

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith({name: '', handle: 'custom-handle'});
      });

      // After manual edit, changing the name should not update the handle
      const nameInput = screen.getByPlaceholderText('e.g. Customer Sign-in');
      fireEvent.change(nameInput, {target: {value: 'New Name'}});

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenLastCalledWith(expect.objectContaining({name: 'New Name'}));
      });
    });
  });

  describe('Suggestion Chips', () => {
    it('should set name and derive the handle when the suggestion is clicked', async () => {
      render(<ConfigureFlowName {...defaultProps} />);

      fireEvent.click(screen.getByText('Cosmic Gateway'));

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith({
          name: 'Cosmic Gateway',
          handle: 'cosmic-gateway',
        });
      });
    });
  });

  describe('Validation', () => {
    it('should call onReadyChange with false initially for empty values', async () => {
      render(<ConfigureFlowName {...defaultProps} />);

      await waitFor(() => {
        expect(mockOnReadyChange).toHaveBeenCalledWith(false);
      });
    });

    it('should render handle helper text', () => {
      render(<ConfigureFlowName {...defaultProps} />);

      expect(screen.getByText('Lowercase letters, numbers, and hyphens only')).toBeInTheDocument();
    });
  });
});
