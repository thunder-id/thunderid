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

import type {EmbeddedFlowComponent} from '@thunderid/react';
import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserInvitePage from '../UserInvitePage';

interface TestInviteUserRenderProps {
  additionalData: Record<string, unknown> | undefined;
  values: Record<string, unknown>;
  fieldErrors: Record<string, string>;
  touched: Record<string, boolean>;
  error: Error | null;
  isLoading: boolean;
  components: EmbeddedFlowComponent[];
  handleInputChange: (name: string, value: unknown) => void;
  handleInputBlur: (name: string) => void;
  handleSubmit: () => Promise<void>;
  resetFlow: () => void;
  isValid: boolean;
  meta: unknown;
}

/* ------------------------------------------------------------------ */
/*  Top-level mock state                                              */
/* ------------------------------------------------------------------ */

const mockNavigate = vi.fn();
const mockLoggerInfo = vi.fn();
const mockLoggerError = vi.fn();

const mockHandleInputChange = vi.fn();
const mockHandleInputBlur = vi.fn();
const mockHandleSubmit = vi.fn().mockResolvedValue(undefined);
const mockResetFlow = vi.fn();

let simulateInviteUserError = false;
const mockInviteUserError = new Error('Invite user failed');

let capturedOnFlowChange: ((response: unknown) => void) | null = null;

const defaultRenderProps: TestInviteUserRenderProps = {
  additionalData: undefined,
  values: {},
  fieldErrors: {},
  touched: {},
  error: null,
  isLoading: false,
  components: [],
  handleInputChange: mockHandleInputChange,
  handleInputBlur: mockHandleInputBlur,
  handleSubmit: mockHandleSubmit,
  resetFlow: mockResetFlow,
  isValid: false,
  meta: null,
};

// Mutable reference the mock reads at render time
let mockInviteUserRenderProps: TestInviteUserRenderProps = {...defaultRenderProps};

/* ------------------------------------------------------------------ */
/*  Mocks                                                             */
/* ------------------------------------------------------------------ */

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    info: mockLoggerInfo,
    error: mockLoggerError,
    debug: vi.fn(),
    warn: vi.fn(),
  }),
}));

vi.mock('react-router', async () => ({
  ...(await vi.importActual<typeof import('react-router')>('react-router')),
  useNavigate: () => mockNavigate,
}));

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useThunderID: () => ({
      resolveFlowTemplateLiterals: (text: string | undefined) => text ?? '',
    }),
    InviteUser: ({
      children,
      onError,
      onFlowChange,
    }: {
      children: (props: TestInviteUserRenderProps) => JSX.Element;
      onError?: (error: Error) => void;
      onFlowChange?: (response: unknown) => void;
    }) => {
      // Capture onFlowChange so tests can invoke it
      capturedOnFlowChange = onFlowChange ?? null;

      if (simulateInviteUserError && onError) {
        setTimeout(() => {
          onError(mockInviteUserError);
        }, 0);
      }
      return children(mockInviteUserRenderProps);
    },
  };
});

vi.mock('@thunderid/hooks', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useTemplateLiteralResolver: () => ({
      resolve: (key: string) => key,
    }),
  };
});

vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitTreePicker: ({
    value,
    onChange,
    rootOuId = undefined,
  }: {
    value: string;
    onChange: (id: string) => void;
    rootOuId?: string;
  }) => (
    <div data-testid="ou-tree-picker" data-value={value} data-root-ou-id={rootOuId}>
      <button type="button" onClick={() => onChange('selected-ou-id')}>
        Select OU
      </button>
    </div>
  ),
}));

/* ------------------------------------------------------------------ */
/*  Helpers                                                           */
/* ------------------------------------------------------------------ */

/** Build a heading component */
const heading = (label: string, id?: string): EmbeddedFlowComponent =>
  ({type: 'TEXT', variant: 'HEADING_1', label, id: id ?? `heading-${label}`}) as unknown as EmbeddedFlowComponent;

/** Build a subtitle component */
const subtitle = (label: string, id?: string): EmbeddedFlowComponent =>
  ({type: 'TEXT', variant: 'HEADING_2', label, id: id ?? `subtitle-${label}`}) as unknown as EmbeddedFlowComponent;

/** Build a text input component */
const textInput = (
  ref: string,
  label: string,
  opts?: {required?: boolean; placeholder?: string; id?: string},
): EmbeddedFlowComponent =>
  ({
    type: 'TEXT_INPUT',
    ref,
    label,
    required: opts?.required ?? false,
    placeholder: opts?.placeholder ?? '',
    id: opts?.id ?? `input-${ref}`,
  }) as unknown as EmbeddedFlowComponent;

/** Build an email input component */
const emailInput = (
  ref: string,
  label: string,
  opts?: {required?: boolean; placeholder?: string; id?: string},
): EmbeddedFlowComponent =>
  ({
    type: 'EMAIL_INPUT',
    ref,
    label,
    required: opts?.required ?? false,
    placeholder: opts?.placeholder ?? '',
    id: opts?.id ?? `email-${ref}`,
  }) as unknown as EmbeddedFlowComponent;

/** Build a phone input component */
const phoneInput = (
  ref: string,
  label: string,
  opts?: {required?: boolean; placeholder?: string; id?: string},
): EmbeddedFlowComponent =>
  ({
    type: 'PHONE_INPUT',
    ref,
    label,
    required: opts?.required ?? false,
    placeholder: opts?.placeholder ?? '',
    id: opts?.id ?? `phone-${ref}`,
  }) as unknown as EmbeddedFlowComponent;

/** Build a password input component */
const passwordInput = (
  ref: string,
  label: string,
  opts?: {required?: boolean; placeholder?: string; id?: string},
): EmbeddedFlowComponent =>
  ({
    type: 'PASSWORD_INPUT',
    ref,
    label,
    required: opts?.required ?? false,
    placeholder: opts?.placeholder ?? '',
    id: opts?.id ?? `password-${ref}`,
  }) as unknown as EmbeddedFlowComponent;

/** Build a select component */
const selectInput = (
  ref: string,
  label: string,
  options: unknown[],
  opts?: {required?: boolean; placeholder?: string; hint?: string; id?: string},
): EmbeddedFlowComponent =>
  ({
    type: 'SELECT',
    ref,
    label,
    options,
    required: opts?.required ?? false,
    placeholder: opts?.placeholder ?? '',
    hint: opts?.hint,
    id: opts?.id ?? `select-${ref}`,
  }) as unknown as EmbeddedFlowComponent;

/** Build an OU select component */
const ouSelect = (ref: string, label: string, opts?: {required?: boolean; id?: string}): EmbeddedFlowComponent =>
  ({
    type: 'OU_SELECT',
    ref,
    label,
    required: opts?.required ?? false,
    id: opts?.id ?? `ou-${ref}`,
  }) as unknown as EmbeddedFlowComponent;

/** Build a submit action component */
const submitAction = (label: string, opts?: {variant?: string; id?: string}): EmbeddedFlowComponent =>
  ({
    type: 'ACTION',
    eventType: 'SUBMIT',
    label,
    variant: opts?.variant ?? 'PRIMARY',
    id: opts?.id ?? `action-${label}`,
  }) as unknown as EmbeddedFlowComponent;

/** Wrap sub-components in a BLOCK */
const block = (children: EmbeddedFlowComponent[], id?: string): EmbeddedFlowComponent =>
  ({
    type: 'BLOCK',
    components: children,
    id: id ?? 'block-1',
  }) as unknown as EmbeddedFlowComponent;

/** Wrap actions in a STACK */
const stack = (
  children: EmbeddedFlowComponent[],
  opts?: {direction?: string; justify?: string; id?: string},
): EmbeddedFlowComponent =>
  ({
    type: 'STACK',
    components: children,
    direction: opts?.direction ?? 'row',
    justify: opts?.justify ?? 'center',
    id: opts?.id ?? 'stack-1',
  }) as unknown as EmbeddedFlowComponent;

/* ------------------------------------------------------------------ */
/*  Tests                                                             */
/* ------------------------------------------------------------------ */

describe('UserInvitePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    simulateInviteUserError = false;
    capturedOnFlowChange = null;
    mockInviteUserRenderProps = {...defaultRenderProps};
    Object.assign(mockInviteUserError, {message: 'Invite user failed', response: undefined});
  });

  /* ----- Loading state ----- */

  describe('loading state', () => {
    it('should show a loading spinner when isLoading is true and there are no components', () => {
      mockInviteUserRenderProps.isLoading = true;
      mockInviteUserRenderProps.components = [];

      render(<UserInvitePage />);

      // LinearProgress (determinate) + CircularProgress (indeterminate) = 2 progressbars
      const progressBars = screen.getAllByRole('progressbar');
      expect(progressBars.length).toBe(2);
    });

    it('should show a loading spinner when components array is empty and not loading', () => {
      mockInviteUserRenderProps.isLoading = false;
      mockInviteUserRenderProps.components = [];

      render(<UserInvitePage />);

      // Falls through to the "no components" branch which also shows CircularProgress
      const progressBars = screen.getAllByRole('progressbar');
      expect(progressBars.length).toBe(2);
    });
  });

  /* ----- Default header ----- */

  describe('header', () => {
    it('should display default "Add User" breadcrumb when no steps have been visited', () => {
      mockInviteUserRenderProps.isLoading = true;
      mockInviteUserRenderProps.components = [];

      render(<UserInvitePage />);

      expect(screen.getByText('Add User')).toBeInTheDocument();
    });

    it('should render a close button that navigates to /users', async () => {
      mockInviteUserRenderProps.isLoading = true;
      mockInviteUserRenderProps.components = [];

      render(<UserInvitePage />);

      const closeButton = screen.getByRole('button', {name: /close/i});
      await userEvent.click(closeButton);

      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });
  });

  /* ----- Form fields rendering ----- */

  describe('form fields rendering', () => {
    it('should render a TEXT_INPUT field', () => {
      mockInviteUserRenderProps.components = [
        heading('Step 1'),
        block([textInput('firstName', 'First Name', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      expect(screen.getByLabelText(/first name/i)).toBeInTheDocument();
    });

    it('should render an EMAIL_INPUT field', () => {
      mockInviteUserRenderProps.components = [
        heading('Email Step'),
        block([emailInput('email', 'Email Address', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      expect(screen.getByLabelText(/email address/i)).toBeInTheDocument();
    });

    it('should render a PHONE_INPUT field', () => {
      mockInviteUserRenderProps.components = [
        heading('Phone Step'),
        block([phoneInput('phone', 'Phone Number', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const input = screen.getByLabelText(/phone number/i);
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('type', 'tel');
    });

    it('should render a PASSWORD_INPUT field', () => {
      mockInviteUserRenderProps.components = [
        heading('Password Step'),
        block([passwordInput('password', 'Password', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const input = screen.getByLabelText(/^password$/i);
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('type', 'password');
    });

    it('should toggle password visibility in PASSWORD_INPUT field', async () => {
      mockInviteUserRenderProps.components = [
        heading('Password Step'),
        block([passwordInput('password', 'Password', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const input = screen.getByLabelText(/^password$/i);
      expect(input).toHaveAttribute('type', 'password');

      // Find and click the toggle button (shows 'show password' when password is hidden)
      const toggleButton = screen.getByLabelText('show password');
      await userEvent.click(toggleButton);

      expect(input).toHaveAttribute('type', 'text');

      // Toggle back (shows 'hide password' when password is visible)
      const hideButton = screen.getByLabelText('hide password');
      await userEvent.click(hideButton);
      expect(input).toHaveAttribute('type', 'password');
    });

    it('should render a SELECT field with options', () => {
      const options = [
        {value: 'admin', label: 'Admin'},
        {value: 'user', label: 'User'},
      ];
      mockInviteUserRenderProps.components = [
        heading('Role Step'),
        block([selectInput('role', 'Role', options, {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      // MUI Select renders a <div> so getByLabelText won't work; check the FormLabel text instead
      expect(screen.getByText('Role')).toBeInTheDocument();
      // The select's combobox role should be present
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should render an OU_SELECT field with OrganizationUnitTreePicker', () => {
      mockInviteUserRenderProps.components = [
        heading('OU Step'),
        block([ouSelect('ou', 'Organization Unit', {required: true}), submitAction('Next')]),
      ];
      mockInviteUserRenderProps.additionalData = {rootOuId: 'root-123'};

      render(<UserInvitePage />);

      const picker = screen.getByTestId('ou-tree-picker');
      expect(picker).toBeInTheDocument();
      expect(picker).toHaveAttribute('data-root-ou-id', 'root-123');
    });

    it('should render a heading component as an h1', () => {
      mockInviteUserRenderProps.components = [
        heading('User Details'),
        block([textInput('name', 'Name'), submitAction('Submit')]),
      ];

      render(<UserInvitePage />);

      // Heading appears in the form (h1) and in the breadcrumb (h5)
      const headings = screen.getAllByText('User Details');
      expect(headings.length).toBeGreaterThanOrEqual(1);
      // The h1 is the form heading
      const h1 = headings.find((el) => el.tagName === 'H1');
      expect(h1).toBeDefined();
    });

    it('should render subtitle text components', () => {
      mockInviteUserRenderProps.components = [
        heading('Step Title'),
        subtitle('Please fill in the details'),
        block([textInput('name', 'Name'), submitAction('Submit')]),
      ];

      render(<UserInvitePage />);

      expect(screen.getByText('Please fill in the details')).toBeInTheDocument();
    });

    it('should not render a block without a submit action', () => {
      mockInviteUserRenderProps.components = [heading('Step Title'), block([textInput('name', 'Name')])];

      render(<UserInvitePage />);

      // The heading should render (appears in form h1 and breadcrumb h5)
      const headings = screen.getAllByText('Step Title');
      expect(headings.length).toBeGreaterThanOrEqual(1);
      // The text input should NOT render because the block has no submit action
      expect(screen.queryByLabelText(/name/i)).not.toBeInTheDocument();
    });
  });

  /* ----- Display-only prompt state ----- */

  describe('display-only prompt state', () => {
    /** Helper: build a COPYABLE_TEXT component */
    const copyableText = (source: string, label?: string, id?: string): EmbeddedFlowComponent =>
      ({
        type: 'COPYABLE_TEXT',
        source,
        label,
        id: id ?? `copyable-${source}`,
      }) as unknown as EmbeddedFlowComponent;

    it('should render TEXT components without a BLOCK (display-only prompt)', () => {
      mockInviteUserRenderProps.components = [
        heading('Invite Sent'),
        {type: 'TEXT', label: 'Check your email.', id: 'msg'} as unknown as EmbeddedFlowComponent,
      ];

      render(<UserInvitePage />);

      expect(screen.getAllByText('Invite Sent').length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText('Check your email.')).toBeInTheDocument();
    });

    it('should show "Close" and "Add Another User" buttons when no BLOCK components present', () => {
      mockInviteUserRenderProps.components = [heading('Done')];

      render(<UserInvitePage />);

      const closeButtons = screen.getAllByRole('button', {name: /close/i});
      expect(closeButtons.length).toBeGreaterThanOrEqual(2); // header X + footer Close
      expect(screen.getByRole('button', {name: /add another user/i})).toBeInTheDocument();
    });

    it('should call resetFlow when "Add Another User" is clicked in display-only state', async () => {
      mockInviteUserRenderProps.components = [heading('Done')];

      render(<UserInvitePage />);

      await userEvent.click(screen.getByRole('button', {name: /add another user/i}));

      expect(mockResetFlow).toHaveBeenCalled();
    });

    it('should navigate to /users when footer Close button is clicked in display-only state', async () => {
      mockInviteUserRenderProps.components = [heading('Done')];

      render(<UserInvitePage />);

      // Footer close button is the last close button
      const closeButtons = screen.getAllByRole('button', {name: /close/i});
      await userEvent.click(closeButtons[closeButtons.length - 1]);

      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });

    it('should render COPYABLE_TEXT component with value from additionalData', () => {
      mockInviteUserRenderProps.components = [
        heading('Invite Link Generated'),
        copyableText('inviteLink', 'Invite Link'),
      ];
      mockInviteUserRenderProps.additionalData = {inviteLink: 'https://example.com/invite/abc123'};

      render(<UserInvitePage />);

      expect(screen.getByText('https://example.com/invite/abc123')).toBeInTheDocument();
    });

    it('should render COPYABLE_TEXT label from component when present', () => {
      mockInviteUserRenderProps.components = [heading('Link Ready'), copyableText('inviteLink', 'Invite Link')];
      mockInviteUserRenderProps.additionalData = {inviteLink: 'https://example.com/invite/xyz'};

      render(<UserInvitePage />);

      expect(screen.getByText('Invite Link')).toBeInTheDocument();
    });

    it('should not show "Add Another User" button when BLOCK components are present', () => {
      mockInviteUserRenderProps.components = [
        heading('Step 1'),
        block([textInput('name', 'Name'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      expect(screen.queryByRole('button', {name: /add another user/i})).not.toBeInTheDocument();
    });
  });

  /* ----- Error states ----- */

  describe('error states', () => {
    it('should show error alert when error is present and no components', () => {
      mockInviteUserRenderProps.error = new Error('Something went wrong');
      mockInviteUserRenderProps.components = [];

      render(<UserInvitePage />);

      expect(screen.getByText('Error')).toBeInTheDocument();
      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    });

    it('should show close button in error state without components', () => {
      mockInviteUserRenderProps.error = new Error('Something went wrong');
      mockInviteUserRenderProps.components = [];

      render(<UserInvitePage />);

      // The inner close button inside the error content
      const closeButtons = screen.getAllByRole('button', {name: /close/i});
      expect(closeButtons.length).toBeGreaterThanOrEqual(2); // header X + inner close
    });

    it('should show error alert alongside form when error is present with components', () => {
      mockInviteUserRenderProps.error = new Error('Validation failed');
      mockInviteUserRenderProps.components = [
        heading('Step 1'),
        block([textInput('name', 'Name'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      expect(screen.getByText('Error')).toBeInTheDocument();
      expect(screen.getByText('Validation failed')).toBeInTheDocument();
      // Form fields should still be visible
      expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
    });

    it('should show flowError from onFlowChange response', async () => {
      mockInviteUserRenderProps.components = [
        heading('Step 1'),
        block([textInput('name', 'Name'), submitAction('Next')]),
      ];

      const {rerender} = render(<UserInvitePage />);

      // Trigger onFlowChange with an error
      if (capturedOnFlowChange) {
        capturedOnFlowChange({
          error: {
            code: 'FEE-60005',
            message: {key: 'flows.errors.user_exists', defaultValue: 'User already exists'},
            description: {key: 'flows.errors.user_exists_desc', defaultValue: 'User already exists'},
          },
        });
      }

      // Re-render to reflect state change
      rerender(<UserInvitePage />);

      await waitFor(() => {
        expect(screen.getByText('User already exists')).toBeInTheDocument();
      });
    });

    it('should call onError callback when simulateInviteUserError is true', async () => {
      simulateInviteUserError = true;

      render(<UserInvitePage />);

      await waitFor(() => {
        expect(mockLoggerError).toHaveBeenCalledWith('User onboarding error', {error: mockInviteUserError});
      });
    });

    it('should fall back to manual user creation when the onboarding flow is missing on error', async () => {
      simulateInviteUserError = true;
      Object.assign(mockInviteUserError, {
        message: 'Flow not found',
        response: {status: 404, data: {code: 'FLM-1003'}},
      });

      render(<UserInvitePage />);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/users/create');
      });

      expect(mockLoggerInfo).toHaveBeenCalledWith(
        'Falling back to manual user creation because the onboarding flow is unavailable',
      );

      simulateInviteUserError = false;
      Object.assign(mockInviteUserError, {message: 'Invite user failed', response: undefined});
    });

    it('should fall back to manual user creation when flow change reports a missing onboarding flow', async () => {
      render(<UserInvitePage />);

      if (capturedOnFlowChange) {
        capturedOnFlowChange({
          error: {
            code: 'FLM-1003',
            message: {key: 'flows.errors.not_found', defaultValue: 'Flow not found'},
            description: {key: 'flows.errors.not_found.desc', defaultValue: 'Flow not found'},
          },
          response: {status: 404, data: {code: 'FLM-1003'}},
        });
      }

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/users/create');
      });
    });
  });

  /* ----- Breadcrumb and progress tracking ----- */

  describe('breadcrumb and progress tracking', () => {
    it('should update breadcrumb when step label changes', async () => {
      mockInviteUserRenderProps.components = [
        heading('Select User Type'),
        block([textInput('type', 'Type'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      await waitFor(() => {
        // Heading appears in form (h1) and breadcrumb (h5)
        const matches = screen.getAllByText('Select User Type');
        expect(matches.length).toBeGreaterThanOrEqual(2);
        // One should be in a breadcrumb (h5)
        const breadcrumbHeading = matches.find((el) => el.tagName === 'H5');
        expect(breadcrumbHeading).toBeDefined();
      });
    });

    it('should render a linear progress bar', () => {
      mockInviteUserRenderProps.components = [
        heading('Step 1'),
        block([textInput('name', 'Name'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      // LinearProgress is rendered at the top
      const progressBars = screen.getAllByRole('progressbar');
      expect(progressBars.length).toBeGreaterThanOrEqual(1);
    });
  });

  /* ----- User interactions ----- */

  describe('user interactions', () => {
    it('should call handleInputChange when typing in a TEXT_INPUT', async () => {
      mockInviteUserRenderProps.components = [
        heading('Details'),
        block([textInput('firstName', 'First Name'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const input = screen.getByLabelText(/first name/i);
      await userEvent.type(input, 'John');

      expect(mockHandleInputChange).toHaveBeenCalled();
    });

    it('should call handleInputChange when typing in a PHONE_INPUT', async () => {
      mockInviteUserRenderProps.components = [
        heading('Phone'),
        block([phoneInput('phone', 'Phone Number', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const input = screen.getByLabelText(/phone number/i);
      await userEvent.type(input, '+1234567890');

      expect(mockHandleInputChange).toHaveBeenCalled();
    });

    it('should call handleInputChange when typing in an EMAIL_INPUT', async () => {
      mockInviteUserRenderProps.components = [
        heading('Email'),
        block([emailInput('email', 'Email', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const input = screen.getByLabelText(/email/i);
      await userEvent.type(input, 'test@example.com');

      expect(mockHandleInputChange).toHaveBeenCalled();
    });

    it('should call handleInputChange when typing in a PASSWORD_INPUT', async () => {
      mockInviteUserRenderProps.components = [
        heading('Password'),
        block([passwordInput('password', 'Password', {required: true}), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const input = screen.getByLabelText(/^password$/i);
      await userEvent.type(input, 'SuperSecret123');

      expect(mockHandleInputChange).toHaveBeenCalled();
    });

    it('should call handleInputChange when selecting an OU', async () => {
      mockInviteUserRenderProps.components = [
        heading('OU Step'),
        block([ouSelect('ou', 'Organization Unit'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      const selectButton = screen.getByText('Select OU');
      await userEvent.click(selectButton);

      expect(mockHandleInputChange).toHaveBeenCalledWith('ou', 'selected-ou-id');
    });

    it('should disable submit button when form is invalid (isValid=false and propsIsValid=false)', () => {
      mockInviteUserRenderProps.isValid = false;
      mockInviteUserRenderProps.components = [
        heading('Step'),
        block([textInput('name', 'Name', {required: true}), submitAction('Submit', {variant: 'PRIMARY'})]),
      ];

      render(<UserInvitePage />);

      const submitButton = screen.getByRole('button', {name: /submit/i});
      expect(submitButton).toBeDisabled();
    });

    it('should enable submit button when both propsIsValid and local form are valid', async () => {
      mockInviteUserRenderProps.isValid = true;
      mockInviteUserRenderProps.components = [
        heading('Step'),
        block([textInput('name', 'Name'), submitAction('Submit', {variant: 'PRIMARY'})]),
      ];

      render(<UserInvitePage />);

      // Wait for react-hook-form to complete initial validation cycle
      await waitFor(() => {
        const submitButton = screen.getByRole('button', {name: /submit/i});
        expect(submitButton).not.toBeDisabled();
      });
    });

    it('should show loading spinner in submit button when isLoading is true with components', () => {
      mockInviteUserRenderProps.isLoading = true;
      mockInviteUserRenderProps.isValid = true;
      mockInviteUserRenderProps.components = [
        heading('Step'),
        block([textInput('name', 'Name'), submitAction('Submit')]),
      ];

      render(<UserInvitePage />);

      // The submit button should contain a CircularProgress
      const submitButton = screen.getByRole('button', {name: ''});
      expect(submitButton).toBeDisabled();
    });
  });

  /* ----- Submit handling ----- */

  describe('form submission', () => {
    it('should call handleSubmit when form is submitted and valid', async () => {
      mockInviteUserRenderProps.isValid = true;
      mockInviteUserRenderProps.values = {name: 'Test User'};
      mockInviteUserRenderProps.components = [
        heading('Step'),
        block([textInput('name', 'Name'), submitAction('Submit', {variant: 'PRIMARY'})]),
      ];

      render(<UserInvitePage />);

      const submitButton = screen.getByRole('button', {name: /submit/i});
      await userEvent.click(submitButton);

      expect(mockHandleSubmit).toHaveBeenCalled();
    });
  });

  /* ----- Progress calculation ----- */

  describe('progress calculation', () => {
    it('should detect OU step and adjust total steps to 5', async () => {
      mockInviteUserRenderProps.components = [
        heading('OU Assignment'),
        block([ouSelect('ou', 'Organization Unit'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      // The OU step detection triggers hasOuStep=true, changing totalSteps to 5
      // With 1 breadcrumb and 5 total steps, progress = 20%
      await waitFor(() => {
        const progressBar = screen.getAllByRole('progressbar')[0];
        expect(progressBar).toHaveAttribute('aria-valuenow', '20');
      });
    });

    it('should calculate progress without OU step as 4 total steps', async () => {
      mockInviteUserRenderProps.components = [
        heading('User Type'),
        block([textInput('type', 'Type'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      // With 1 breadcrumb and 4 total steps, progress = 25%
      await waitFor(() => {
        const progressBar = screen.getAllByRole('progressbar')[0];
        const value = Number(progressBar.getAttribute('aria-valuenow'));
        expect(value).toBeCloseTo(25, 0);
      });
    });
  });

  /* ----- OU step in nested block ----- */

  describe('OU detection in nested blocks', () => {
    it('should detect OU_SELECT within block sub-components', async () => {
      mockInviteUserRenderProps.components = [
        heading('Assign OU'),
        block([ouSelect('orgUnit', 'Unit'), submitAction('Next')]),
      ];

      render(<UserInvitePage />);

      await waitFor(() => {
        const progressBar = screen.getAllByRole('progressbar')[0];
        // With OU detected, totalSteps=5, 1 breadcrumb -> 20%
        expect(progressBar).toHaveAttribute('aria-valuenow', '20');
      });
    });
  });

  /* ----- STACK rendering ----- */

  describe('STACK inside BLOCK', () => {
    it('should render multiple action buttons from a STACK', () => {
      mockInviteUserRenderProps.isValid = true;
      mockInviteUserRenderProps.components = [
        heading('Choose Delivery'),
        block([stack([submitAction('Send Email', {id: 'act-email'}), submitAction('Get Link', {id: 'act-link'})])]),
      ];

      render(<UserInvitePage />);

      expect(screen.getByRole('button', {name: /send email/i})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /get link/i})).toBeInTheDocument();
    });

    it('should call handleSubmit with the clicked STACK action', async () => {
      mockInviteUserRenderProps.isValid = true;
      mockInviteUserRenderProps.components = [
        heading('Choose Delivery'),
        block([stack([submitAction('Send Email', {id: 'act-email'}), submitAction('Get Link', {id: 'act-link'})])]),
      ];

      render(<UserInvitePage />);

      await userEvent.click(screen.getByRole('button', {name: /send email/i}));

      expect(mockHandleSubmit).toHaveBeenCalled();
    });

    it('should use the first STACK action as the primary action for form submission', () => {
      mockInviteUserRenderProps.isValid = true;
      mockInviteUserRenderProps.components = [
        heading('Choose Delivery'),
        block([stack([submitAction('Send Email', {id: 'act-email'}), submitAction('Get Link', {id: 'act-link'})])]),
      ];

      render(<UserInvitePage />);

      // Form submit (Enter key) should use the primary action derived from the nested STACK
      const form = screen.getByRole('button', {name: /send email/i}).closest('form');
      expect(form).not.toBeNull();
    });

    it('should disable all STACK buttons when isLoading is true', () => {
      mockInviteUserRenderProps.isLoading = true;
      mockInviteUserRenderProps.isValid = true;
      mockInviteUserRenderProps.components = [
        heading('Choose Delivery'),
        block([stack([submitAction('Send Email', {id: 'act-email'}), submitAction('Get Link', {id: 'act-link'})])]),
      ];

      render(<UserInvitePage />);

      expect(screen.getByRole('button', {name: /send email/i})).toBeDisabled();
      expect(screen.getByRole('button', {name: /get link/i})).toBeDisabled();
    });

    it('should show spinner only on the clicked STACK action button while loading', async () => {
      // Start not loading so we can click; mock will stay resolved
      mockInviteUserRenderProps.isValid = true;

      // Simulate in-flight loading state by making handleSubmit never resolve during this test
      let resolveSubmit!: () => void;
      mockHandleSubmit.mockImplementationOnce(
        () =>
          new Promise<void>((res) => {
            resolveSubmit = res;
          }),
      );

      mockInviteUserRenderProps.components = [
        heading('Choose Delivery'),
        block([stack([submitAction('Send Email', {id: 'act-email'}), submitAction('Get Link', {id: 'act-link'})])]),
      ];

      // Render with loading=false first so buttons are enabled
      const {rerender} = render(<UserInvitePage />);

      // After clicking, re-render with isLoading=true to simulate the SDK entering loading state
      await userEvent.click(screen.getByRole('button', {name: /send email/i}));

      mockInviteUserRenderProps = {...mockInviteUserRenderProps, isLoading: true};
      rerender(<UserInvitePage />);

      // Both buttons are disabled while loading
      expect(screen.getByRole('button', {name: /get link/i})).toBeDisabled();
      // "Get Link" (not clicked) still shows its label
      expect(screen.getByRole('button', {name: /get link/i})).toHaveTextContent('Get Link');
      // "Send Email" (clicked) is disabled and shows no visible label (spinner replaces it)
      const sendEmailButtons = screen.getAllByRole('button');
      const sendEmailButton = sendEmailButtons.find(
        (btn) => btn.getAttribute('disabled') !== null && !btn.textContent?.includes('Get Link'),
      );
      expect(sendEmailButton).toBeDefined();
      expect(sendEmailButton).toBeDisabled();

      resolveSubmit();
    });

    it('should not render a BLOCK when STACK has no submit actions', () => {
      mockInviteUserRenderProps.components = [
        heading('Choose'),
        // STACK contains non-submit components, no direct submit actions either
        block([stack([{type: 'TEXT', label: 'info', id: 'txt-info'} as unknown as EmbeddedFlowComponent])]),
      ];

      render(<UserInvitePage />);

      // Block should not render (no primary action found)
      expect(screen.queryByRole('form')).not.toBeInTheDocument();
    });
  });

  /* ----- Clearing flow error on reset ----- */

  describe('flow error reset', () => {
    it('should clear flowError when onFlowChange receives no error', async () => {
      mockInviteUserRenderProps.components = [
        heading('Step'),
        block([textInput('name', 'Name'), submitAction('Next')]),
      ];

      const {rerender} = render(<UserInvitePage />);

      // First set an error
      if (capturedOnFlowChange) {
        capturedOnFlowChange({
          error: {
            code: 'FEE-60001',
            message: {key: 'flows.errors.some', defaultValue: 'Some error'},
            description: {key: 'flows.errors.some.desc', defaultValue: 'Some error'},
          },
        });
      }
      rerender(<UserInvitePage />);

      await waitFor(() => {
        expect(screen.getByText('Some error')).toBeInTheDocument();
      });

      // Then clear it
      if (capturedOnFlowChange) {
        capturedOnFlowChange({});
      }
      rerender(<UserInvitePage />);

      await waitFor(() => {
        expect(screen.queryByText('Some error')).not.toBeInTheDocument();
      });
    });
  });
});
