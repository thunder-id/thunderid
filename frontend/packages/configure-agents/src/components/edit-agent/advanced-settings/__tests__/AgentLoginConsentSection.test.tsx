// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent} from '@testing-library/react';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent, OAuthAgentConfig} from '../../../../models/agent';
import AgentLoginConsentSection from '../AgentLoginConsentSection';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('AgentLoginConsentSection', () => {
  const mockOnFieldChange = vi.fn();

  const agent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    display: 'Test Agent',
  };

  const userFacingConfig: OAuthAgentConfig = {
    grantTypes: ['authorization_code'],
    responseTypes: ['code'],
  };

  const getField = (): HTMLElement => screen.getByRole('spinbutton', {name: 'Validity period'});

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns null when oauth2Config is undefined', () => {
    const {container} = render(
      <AgentLoginConsentSection agent={agent} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it.each([
    ['client credentials only', ['client_credentials']],
    [
      'client credentials and token exchange',
      ['client_credentials', 'urn:ietf:params:oauth:grant-type:token-exchange'],
    ],
  ])('returns null for an agent with %s', (_label, grantTypes) => {
    const {container} = render(
      <AgentLoginConsentSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={{grantTypes, responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it.each([
    ['authorization code', 'authorization_code'],
    ['CIBA', 'urn:openid:params:grant-type:ciba'],
  ])('renders the card for an agent with the %s grant', (_label, grant) => {
    render(
      <AgentLoginConsentSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={{grantTypes: [grant], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByText('Consent validity')).toBeInTheDocument();
    expect(getField()).toHaveValue(null);
  });

  it('shows the saved validity period', () => {
    render(
      <AgentLoginConsentSection
        agent={{...agent, loginConsent: {validityPeriod: 3600}}}
        editedAgent={{}}
        oauth2Config={userFacingConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(getField()).toHaveValue(3600);
  });

  it('prefers the edited validity period over the saved one', () => {
    render(
      <AgentLoginConsentSection
        agent={{...agent, loginConsent: {validityPeriod: 3600}}}
        editedAgent={{loginConsent: {validityPeriod: 120}}}
        oauth2Config={userFacingConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(getField()).toHaveValue(120);
  });

  it('stages loginConsent when a validity period is entered', () => {
    render(
      <AgentLoginConsentSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={userFacingConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    fireEvent.change(getField(), {target: {value: '3600'}});

    expect(mockOnFieldChange).toHaveBeenCalledWith('loginConsent', {validityPeriod: 3600});
  });

  it('disables the field for a read-only agent', () => {
    render(
      <AgentLoginConsentSection
        agent={{...agent, isReadOnly: true}}
        editedAgent={{}}
        oauth2Config={userFacingConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(getField()).toBeDisabled();
  });

  it('reports a validity period it could not commit', () => {
    const onValidationChange = vi.fn();
    render(
      <AgentLoginConsentSection
        agent={{...agent, loginConsent: {validityPeriod: 3600}}}
        editedAgent={{}}
        oauth2Config={userFacingConfig}
        onFieldChange={mockOnFieldChange}
        onValidationChange={onValidationChange}
      />,
    );

    fireEvent.change(getField(), {target: {value: '-1'}});

    expect(mockOnFieldChange).not.toHaveBeenCalled();
    expect(onValidationChange).toHaveBeenLastCalledWith(true);
  });
});
