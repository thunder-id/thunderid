// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {Application} from '@thunderid/configure-applications';
import {describe, it, expect, vi} from 'vitest';
import type {Agent} from '../../../../models/agent';
import EditFlowsSettings from '../EditFlowsSettings';

vi.mock('../../../../../applications/components/edit-application/flows-settings/AuthenticationFlowSection', () => ({
  default: ({application}: {application: Application}) => (
    <div data-testid="auth-flow" data-readonly={String(application.isReadOnly)} />
  ),
}));
vi.mock('../../../../../applications/components/edit-application/flows-settings/RegistrationFlowSection', () => ({
  default: ({application}: {application: Application}) => (
    <div data-testid="registration-flow" data-readonly={String(application.isReadOnly)} />
  ),
}));
// Mocked so the absence assertions below fail if the section is ever wired back in.
vi.mock('../../../../../applications/components/edit-application/flows-settings/RecoveryFlowSection', () => ({
  default: ({application}: {application: Application}) => (
    <div data-testid="recovery-flow" data-readonly={String(application.isReadOnly)} />
  ),
}));

describe('EditFlowsSettings', () => {
  const mockOnFieldChange = vi.fn();
  const baseAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};

  it('renders the authentication and registration flow sections', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toBeInTheDocument();
    expect(screen.getByTestId('registration-flow')).toBeInTheDocument();
  });

  it('never renders the recovery flow section, since the agent API does not persist it', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.queryByTestId('recovery-flow')).not.toBeInTheDocument();
  });

  it('keeps the recovery flow section hidden when Delegated mode is off', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.queryByTestId('recovery-flow')).not.toBeInTheDocument();
  });

  it('keeps flows editable and hides the lock notice when Delegated mode is on', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toHaveAttribute('data-readonly', 'false');
    expect(screen.queryByText(/These settings are frozen for this agent/)).not.toBeInTheDocument();
  });

  it('forces the flow sections read-only and shows the lock notice when Delegated mode is off', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toHaveAttribute('data-readonly', 'true');
    expect(screen.getByTestId('registration-flow')).toHaveAttribute('data-readonly', 'true');
    expect(screen.getByText(/These settings are frozen for this agent/)).toBeInTheDocument();
  });

  it('stays read-only when the agent is already read-only, even with Delegated mode on', () => {
    render(
      <EditFlowsSettings
        agent={{...baseAgent, isReadOnly: true}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toHaveAttribute('data-readonly', 'true');
  });
});
