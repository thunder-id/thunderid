// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {InviteUserRenderProps, EmbeddedFlowComponent} from '@thunderid/react';
import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserCreatePage from '../UserCreatePage';

const mockNavigate = vi.fn();
const mockHandleSubmit = vi.fn();
const mockHandleInputChange = vi.fn();

// Mutable form values the InviteUser mock reads at render time
let mockValues: Record<string, string> = {};

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    Link: ({to, children = undefined, ...props}: {to: string; children?: ReactNode; [key: string]: unknown}) => (
      <a
        {...(props as Record<string, unknown>)}
        href={to}
        onClick={(e) => {
          e.preventDefault();
          Promise.resolve(mockNavigate(to)).catch(() => null);
        }}
      >
        {children}
      </a>
    ),
  };
});

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
    withComponent: vi.fn().mockReturnThis(),
  }),
}));

// Mock InviteUser to provide embedded flow components
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    InviteUser: ({children}: {children: (props: InviteUserRenderProps) => ReactNode}) => {
      const mockComponents: EmbeddedFlowComponent[] = [
        {
          id: 'step-heading',
          type: 'TEXT',
          label: 'User Details',
          ref: undefined,
          components: undefined,
          actions: undefined,
        },
        {
          id: 'email-field',
          type: 'EMAIL_INPUT',
          label: 'Email',
          ref: 'email',
          components: undefined,
          actions: undefined,
        },
        {
          id: 'active-field',
          type: 'BOOLEAN_INPUT',
          label: 'Active',
          ref: 'active',
          required: true,
          components: undefined,
          actions: undefined,
        },
        {
          id: 'submit-btn',
          type: 'ACTION',
          label: 'Create User',
          variant: 'PRIMARY',
          ref: undefined,
          components: undefined,
          actions: undefined,
        },
      ] as EmbeddedFlowComponent[];

      const renderProps: InviteUserRenderProps = {
        components: mockComponents,
        values: mockValues,
        fieldErrors: {},
        touched: {},
        error: null,
        isLoading: false,
        handleInputChange: mockHandleInputChange,
        handleInputBlur: vi.fn(),
        handleSubmit: mockHandleSubmit,
        resetFlow: vi.fn(),
        isValid: true,
        meta: null,
        additionalData: {rootOuId: 'root-ou'},
      };

      return children(renderProps);
    },
  };
});

describe('UserCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockValues = {};
    mockNavigate.mockResolvedValue(undefined);
    mockHandleSubmit.mockResolvedValue(undefined);
  });

  it('renders the page with progress bar', () => {
    render(<UserCreatePage />);

    const progressBars = screen.getAllByRole('progressbar');
    expect(progressBars.length).toBeGreaterThan(0);
  });

  it('renders close button', () => {
    render(<UserCreatePage />);

    expect(screen.getByLabelText('Close')).toBeInTheDocument();
  });

  it('renders breadcrumb container', () => {
    render(<UserCreatePage />);

    // Check that breadcrumb container is rendered
    const breadcrumbContainer = screen.getByLabelText('breadcrumb');
    expect(breadcrumbContainer).toBeInTheDocument();
  });

  it('renders embedded flow components', () => {
    render(<UserCreatePage />);

    expect(screen.getByLabelText('Email')).toBeInTheDocument();
  });

  it('renders Create User action button', () => {
    render(<UserCreatePage />);

    const buttons = screen.getAllByRole('button').filter((btn) => btn.textContent?.includes('Create User'));
    expect(buttons.length).toBeGreaterThan(0);
  });

  it('closes page when X button is clicked', async () => {
    const user = userEvent.setup();
    render(<UserCreatePage />);

    const closeButton = screen.getByLabelText('Close');
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });
  });

  it('renders email input field', () => {
    render(<UserCreatePage />);

    const emailInput = screen.getByLabelText('Email');
    expect(emailInput).toBeInTheDocument();
  });

  it('allows typing in form fields', async () => {
    const user = userEvent.setup();
    render(<UserCreatePage />);

    const emailInput = screen.getByLabelText('Email');
    // User action should complete without error
    await user.type(emailInput, 'test@example.com');

    // Email input field should remain in the document after typing
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
  });

  it('handles form submission', async () => {
    const user = userEvent.setup();
    render(<UserCreatePage />);

    const submitButtons = screen.getAllByRole('button').filter((btn) => btn.textContent?.includes('Create User'));
    expect(submitButtons.length).toBeGreaterThan(0);
    await user.click(submitButtons[0]);

    // handleSubmit should have been called
    await waitFor(() => {
      expect(mockHandleSubmit).toHaveBeenCalled();
    });
  });

  it('displays auto-submit behavior when create action is detected', () => {
    render(<UserCreatePage />);

    // The page should render without errors
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders with AdditionalData containing rootOuId', () => {
    render(<UserCreatePage />);

    // The page should successfully render with the mocked additional data
    expect(screen.getByLabelText('Close')).toBeInTheDocument();
  });

  it('handles translation of form labels', () => {
    render(<UserCreatePage />);

    // Email label should be translated and visible
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
  });

  describe('boolean attributes', () => {
    it('renders a boolean attribute as a checkbox rather than a text field', () => {
      render(<UserCreatePage />);

      expect(screen.getByRole('checkbox', {name: 'Active'})).toBeInTheDocument();
    });

    it('seeds a boolean attribute with its unchecked value', async () => {
      render(<UserCreatePage />);

      await waitFor(() => {
        expect(mockHandleInputChange).toHaveBeenCalledWith('active', 'false');
      });
    });

    it('reports a checked boolean attribute as true', async () => {
      const user = userEvent.setup();
      mockValues = {active: 'false'};
      render(<UserCreatePage />);

      await user.click(screen.getByRole('checkbox', {name: 'Active'}));

      expect(mockHandleInputChange).toHaveBeenCalledWith('active', 'true');
    });

    it('reflects a boolean attribute that is already true', () => {
      mockValues = {active: 'true'};
      render(<UserCreatePage />);

      expect(screen.getByRole('checkbox', {name: 'Active'})).toBeChecked();
    });
  });
});
