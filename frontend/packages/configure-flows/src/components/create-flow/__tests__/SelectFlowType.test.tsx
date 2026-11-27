// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, within} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {FlowTemplate, TemplateAudiences} from '../../../models/templates';
import SelectFlowType from '../SelectFlowType';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (_key: string, defaultValue: string) => defaultValue,
  }),
}));

// Mock oxygen-ui-icons-react
vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    KeyRound: ({size}: {size: number}) => <span data-testid="icon-key-round">{size}</span>,
    ShieldCheck: ({size}: {size: number}) => <span data-testid="icon-shield-check">{size}</span>,
    UserCheck: ({size}: {size: number}) => <span data-testid="icon-user-check">{size}</span>,
    UserCog: ({size}: {size: number}) => <span data-testid="icon-user-cog">{size}</span>,
    UserPlus: ({size}: {size: number}) => <span data-testid="icon-user-plus">{size}</span>,
  };
});

const template = (flowType: FlowTemplate['flowType'], type: string, audience: TemplateAudiences): FlowTemplate => ({
  resourceType: 'TEMPLATE',
  category: 'PASSWORD',
  type,
  flowType,
  audience,
  display: {label: type, image: '', showOnResourcePanel: true},
  config: {name: '', handle: '', nodes: []},
});

// Mirrors how the shipped templates are labelled: blanks, sign-in, device authorization and the personal assistant
// sign-up are written for everyone, and a flow type is offered to an audience only through its other templates.
const mockTemplates: FlowTemplate[] = [
  template('AUTHENTICATION', 'BLANK', 'ALL'),
  template('AUTHENTICATION', 'CREDENTIALS_AUTH', 'ALL'),
  template('AUTHENTICATION', 'BASIC_WITH_CONSENT', 'ALL'),
  template('REGISTRATION', 'BLANK', 'ALL'),
  template('REGISTRATION', 'BASIC', 'USER'),
  template('REGISTRATION', 'PERSONAL_AGENT', 'ALL'),
  template('RECOVERY', 'BLANK', 'ALL'),
  template('RECOVERY', 'Email_Link', 'USER'),
  template('SIGNOUT', 'BLANK', 'ALL'),
  template('SIGNOUT', 'BASIC', 'USER'),
  template('ADMINISTRATION', 'BLANK', 'ALL'),
  template('ADMINISTRATION', 'AGENT_ONBOARDING_OBO', 'AGENT'),
  template('INVITATION', 'DEFAULT', 'USER'),
  template('DEVICE_GRANT', 'DEVICE_GRANT', 'ALL'),
  template('SELF', 'BLANK', 'ALL'),
  template('SELF', 'SELF_ATTRIBUTE_VERIFICATION', 'USER'),
  template('ADMIN_INITIATED', 'ADMIN_ATTRIBUTE_VERIFICATION_EMAIL_LINK', 'USER'),
];

vi.mock('../../../api/useGetFlowsMeta', () => ({
  default: () => ({
    data: {templates: mockTemplates, steps: [], actions: [], elements: [], widgets: [], executors: []},
    error: null,
    isLoading: false,
  }),
}));

describe('SelectFlowType', () => {
  const mockOnTypeChange = vi.fn();
  const mockOnReadyChange = vi.fn();
  const mockOnAudienceChange = vi.fn();

  const defaultProps = {
    selectedType: null,
    onTypeChange: mockOnTypeChange,
    onReadyChange: mockOnReadyChange,
    audience: 'ALL' as const,
    onAudienceChange: mockOnAudienceChange,
  };

  const audienceButton = (name: string): HTMLElement =>
    within(screen.getByRole('group', {name: 'Filter by audience'})).getByRole('button', {name});

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the component with data-testid', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
    });

    it('should render the title', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('What kind of flow do you want to create?')).toBeInTheDocument();
    });

    it('should render Sign-in option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Sign-in')).toBeInTheDocument();
      expect(screen.getByText('Authenticate users with passwords, passkeys, or social providers')).toBeInTheDocument();
    });

    it('should render Self Sign-up option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Self sign-up')).toBeInTheDocument();
      expect(screen.getByText('Let users register themselves with your application')).toBeInTheDocument();
    });

    it('should render icons for both options', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getAllByTestId('icon-key-round')).toHaveLength(2);
      expect(screen.getByTestId('icon-user-plus')).toBeInTheDocument();
    });

    it('should render SignOut option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Sign out')).toBeInTheDocument();
      expect(screen.getByText('Confirm and terminate an established SSO session')).toBeInTheDocument();
    });

    it('should render Administration option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Administration')).toBeInTheDocument();
      expect(screen.getByText('Perform authenticated administrative and security operations')).toBeInTheDocument();
      expect(screen.getByTestId('icon-user-cog')).toBeInTheDocument();
    });

    it('should render the Self option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Self')).toBeInTheDocument();
      expect(
        screen.getByText('Let signed-in users manage their own account, such as verifying an email address'),
      ).toBeInTheDocument();
      expect(screen.getByTestId('icon-user-check')).toBeInTheDocument();
    });

    it('should render the Administrator-initiated option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Administrator-initiated')).toBeInTheDocument();
      expect(
        screen.getByText('Let administrators start an action for a user, such as verifying their email address'),
      ).toBeInTheDocument();
      expect(screen.getByTestId('icon-shield-check')).toBeInTheDocument();
    });
  });

  describe('Audience filter', () => {
    const allLabels = [
      'Sign-in',
      'Self sign-up',
      'Password recovery',
      'Sign out',
      'Administration',
      'Invitation',
      'Device authorization',
      'Self',
      'Administrator-initiated',
    ];

    it('should render the audience choices', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(audienceButton('All')).toBeInTheDocument();
      expect(audienceButton('Users')).toBeInTheDocument();
      expect(audienceButton('Agents')).toBeInTheDocument();
    });

    it('should mark the active audience as pressed', () => {
      render(<SelectFlowType {...defaultProps} audience="USER" />);

      expect(audienceButton('Users')).toHaveAttribute('aria-pressed', 'true');
      expect(audienceButton('All')).toHaveAttribute('aria-pressed', 'false');
    });

    it('should offer every flow type when the audience is All', () => {
      render(<SelectFlowType {...defaultProps} audience="ALL" />);

      allLabels.forEach((label) => expect(screen.getByText(label)).toBeInTheDocument());
    });

    it('should hide a flow type whose only non-blank templates are for agents when the audience is Users', () => {
      render(<SelectFlowType {...defaultProps} audience="USER" />);

      expect(screen.queryByText('Administration')).not.toBeInTheDocument();
      allLabels
        .filter((label) => label !== 'Administration')
        .forEach((label) => expect(screen.getByText(label)).toBeInTheDocument());
    });

    it('should offer only the flow types with a template for agents when the audience is Agents', () => {
      render(<SelectFlowType {...defaultProps} audience="AGENT" />);

      ['Sign-in', 'Self sign-up', 'Administration', 'Device authorization'].forEach((label) =>
        expect(screen.getByText(label)).toBeInTheDocument(),
      );
      ['Password recovery', 'Sign out', 'Invitation', 'Self', 'Administrator-initiated'].forEach((label) =>
        expect(screen.queryByText(label)).not.toBeInTheDocument(),
      );
    });

    it('should not count a blank template as a reason to offer a flow type', () => {
      render(<SelectFlowType {...defaultProps} audience="AGENT" />);

      // Password recovery has a blank template written for everyone, and nothing else for agents.
      expect(screen.queryByText('Password recovery')).not.toBeInTheDocument();
    });

    it('should report the audience the user picks', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(audienceButton('Agents'));

      expect(mockOnAudienceChange).toHaveBeenCalledWith('AGENT');
    });

    it('should not report a change when the active audience is clicked again', () => {
      render(<SelectFlowType {...defaultProps} audience="AGENT" />);

      fireEvent.click(audienceButton('Agents'));

      expect(mockOnAudienceChange).not.toHaveBeenCalled();
    });

    it('should not report readiness on mount when no flow type is selected', () => {
      render(<SelectFlowType {...defaultProps} audience="AGENT" />);

      expect(mockOnReadyChange).not.toHaveBeenCalled();
    });

    it('should report the step as ready while the selected flow type is offered', () => {
      render(<SelectFlowType {...defaultProps} selectedType="ADMINISTRATION" audience="AGENT" />);

      expect(mockOnReadyChange).toHaveBeenLastCalledWith(true);
    });

    it('should report the step as not ready when the audience hides the selected flow type', () => {
      const {rerender} = render(<SelectFlowType {...defaultProps} selectedType="ADMINISTRATION" audience="AGENT" />);

      rerender(<SelectFlowType {...defaultProps} selectedType="ADMINISTRATION" audience="USER" />);

      expect(mockOnReadyChange).toHaveBeenLastCalledWith(false);
    });

    it('should report the step as ready again when the audience offers the selected flow type once more', () => {
      const {rerender} = render(<SelectFlowType {...defaultProps} selectedType="ADMINISTRATION" audience="USER" />);

      rerender(<SelectFlowType {...defaultProps} selectedType="ADMINISTRATION" audience="ALL" />);

      expect(mockOnReadyChange).toHaveBeenLastCalledWith(true);
    });
  });

  describe('Selection', () => {
    it('should call onTypeChange and onReadyChange when a flow type is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Sign-in'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('AUTHENTICATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with SIGNOUT when SignOut is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Sign out'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('SIGNOUT');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with REGISTRATION when Self Sign-up is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Self sign-up'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('REGISTRATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with INVITATION when Invitation is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Invitation'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('INVITATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with ADMINISTRATION when Administration is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Administration'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('ADMINISTRATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with SELF when Self is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Self'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('SELF');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with ADMIN_INITIATED when Administrator-initiated is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Administrator-initiated'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('ADMIN_INITIATED');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should visually indicate the selected type via the card action area', () => {
      const {rerender} = render(<SelectFlowType {...defaultProps} selectedType="AUTHENTICATION" />);

      // The component renders two cards; verify the selected one renders without error
      expect(screen.getByText('Sign-in')).toBeInTheDocument();

      rerender(<SelectFlowType {...defaultProps} selectedType="REGISTRATION" />);

      expect(screen.getByText('Self sign-up')).toBeInTheDocument();
    });

    it('should call onTypeChange and onReadyChange when Enter is pressed on a card', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.keyDown(screen.getByText('Sign-in').closest('[role="button"]')!, {key: 'Enter'});

      expect(mockOnTypeChange).toHaveBeenCalledWith('AUTHENTICATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange and onReadyChange when Space is pressed on a card', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.keyDown(screen.getByText('Sign-in').closest('[role="button"]')!, {key: ' '});

      expect(mockOnTypeChange).toHaveBeenCalledWith('AUTHENTICATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should ignore unrelated key presses on a card', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.keyDown(screen.getByText('Sign-in').closest('[role="button"]')!, {key: 'a'});

      expect(mockOnTypeChange).not.toHaveBeenCalled();
    });

    it('should allow changing selection from one type to another', () => {
      render(<SelectFlowType {...defaultProps} selectedType="AUTHENTICATION" />);

      fireEvent.click(screen.getByText('Self sign-up'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('REGISTRATION');
    });
  });
});
