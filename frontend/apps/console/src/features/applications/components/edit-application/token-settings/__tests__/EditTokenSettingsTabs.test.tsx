// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {Application, OAuth2Config} from '@thunderid/configure-applications';
import {render, screen} from '@thunderid/test-utils';
import {useState} from 'react';
import {describe, it, expect, vi} from 'vitest';
import EditTokenSettingsTabs from '../EditTokenSettingsTabs';

vi.mock('../ClientAccessTokenSection', () => ({
  default: vi.fn(({subjectType}: {subjectType?: string}) => {
    const [clicks, setClicks] = useState(0);
    return (
      <div data-testid="client-token-section" data-subject-type={subjectType}>
        Clicks: {clicks}
        <button type="button" data-testid="client-token-section-bump" onClick={() => setClicks((c) => c + 1)}>
          Bump
        </button>
      </div>
    );
  }),
}));
vi.mock('../EditTokenSettings', () => ({
  default: vi.fn(({sectionResetKey}: {sectionResetKey?: number}) => {
    const [clicks, setClicks] = useState(0);
    return (
      <div data-testid="user-token-section">
        Clicks: {clicks}, Key: {sectionResetKey}
        <button type="button" data-testid="user-token-section-bump" onClick={() => setClicks((c) => c + 1)}>
          Bump
        </button>
      </div>
    );
  }),
}));

const application: Application = {id: 'app-1', name: 'Test App'};
const clientLockMessage = /does not receive tokens for itself/i;
const userLockMessage = /does not receive tokens for signed-in users/i;

/**
 * Builds an OAuth2 config shaped like one loaded from the API, where the backend has always
 * materialized the access token and ID token blocks.
 */
const oauthConfig = (grantTypes: string[]): OAuth2Config => ({
  grantTypes,
  responseTypes: [],
  token: {
    accessToken: {userConfig: {validityPeriod: 3600, attributes: []}},
    idToken: {validityPeriod: 3600, userAttributes: []},
  },
});

describe('EditTokenSettingsTabs', () => {
  const onFieldChange = vi.fn();

  it('renders Application and User audiences in a side selector', () => {
    // The selector only appears when both audiences apply; a single applicable audience is shown
    // on its own.
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['client_credentials', 'authorization_code'])}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.getByRole('tablist', {name: 'Issued to'})).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'Application'})).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'User'})).toBeInTheDocument();
    expect(screen.getByText('M2M access token')).toBeInTheDocument();
    expect(screen.getByText('Tokens for a signed-in user')).toBeInTheDocument();
    expect(screen.getByText(/configured independently for each audience/i)).toBeInTheDocument();
  });

  it('shows the Application settings alone when only client_credentials is granted', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['client_credentials'])}
        onFieldChange={onFieldChange}
      />,
    );

    // Only one audience applies, so there is no picker and no locked entry to explain.
    expect(screen.queryByRole('tablist', {name: 'Issued to'})).not.toBeInTheDocument();
    expect(screen.getByTestId('client-token-section')).toBeInTheDocument();
    expect(screen.queryByText(clientLockMessage)).not.toBeInTheDocument();
    expect(screen.queryByText(userLockMessage)).not.toBeInTheDocument();
  });

  // The application tab must declare its own identity class, not the agent value.
  it('tells the client section its subject type is application', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['client_credentials'])}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.getByTestId('client-token-section')).toHaveAttribute('data-subject-type', 'application');
  });

  it('omits the Application audience when client_credentials is not granted', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['authorization_code'])}
        onFieldChange={onFieldChange}
      />,
    );

    // An audience that does not apply is dropped rather than shown locked behind its own tab.
    expect(screen.queryByRole('tablist', {name: 'Issued to'})).not.toBeInTheDocument();
    expect(screen.queryByText(clientLockMessage)).not.toBeInTheDocument();
    expect(screen.queryByTestId('client-token-section')).not.toBeInTheDocument();
    expect(screen.getByTestId('user-token-section')).toBeInTheDocument();
  });

  it('omits the User audience when no user-facing grant is present', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['client_credentials'])}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.queryByRole('tablist', {name: 'Issued to'})).not.toBeInTheDocument();
    expect(screen.queryByText(userLockMessage)).not.toBeInTheDocument();
    expect(screen.queryByTestId('user-token-section')).not.toBeInTheDocument();
  });

  it('keeps both audiences selectable when each applies', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['client_credentials', 'authorization_code'])}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.getByRole('tab', {name: 'Application'})).toBeEnabled();
    expect(screen.getByRole('tab', {name: 'User'})).toBeEnabled();
  });

  it('opens the User settings when a user-facing grant is present', async () => {
    const user = userEvent.setup();
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['client_credentials', 'authorization_code'])}
        onFieldChange={onFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(screen.getByTestId('user-token-section')).toBeInTheDocument();
    expect(screen.queryByText(userLockMessage)).not.toBeInTheDocument();
  });

  it('shows the User settings alone when the Application audience does not apply', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={oauthConfig(['authorization_code'])}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.queryByRole('tablist', {name: 'Issued to'})).not.toBeInTheDocument();
    expect(screen.getByTestId('user-token-section')).toBeInTheDocument();
  });

  describe('app-native application (no OAuth2 configuration)', () => {
    it('shows the User settings so the assertion config can be edited', () => {
      render(
        <EditTokenSettingsTabs
          application={application}
          editedApp={{}}
          oauth2Config={undefined}
          onFieldChange={onFieldChange}
        />,
      );

      expect(screen.getByTestId('user-token-section')).toBeInTheDocument();
      expect(screen.queryByText(userLockMessage)).not.toBeInTheDocument();
    });

    it('omits the Application audience entirely', () => {
      render(
        <EditTokenSettingsTabs
          application={application}
          editedApp={{}}
          oauth2Config={undefined}
          onFieldChange={onFieldChange}
        />,
      );

      // An app-native application receives no tokens for itself, so that audience is not offered.
      expect(screen.queryByRole('tablist', {name: 'Issued to'})).not.toBeInTheDocument();
      expect(screen.queryByText(clientLockMessage)).not.toBeInTheDocument();
    });
  });

  it('remounts ClientAccessTokenSection when sectionResetKey changes', async () => {
    const user = userEvent.setup();
    const {rerender} = render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={onFieldChange}
        sectionResetKey={0}
      />,
    );

    await user.click(screen.getByTestId('client-token-section-bump'));
    expect(screen.getByTestId('client-token-section')).toHaveTextContent('Clicks: 1');

    rerender(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={onFieldChange}
        sectionResetKey={1}
      />,
    );

    // A fresh key means a fresh mount, so the local click count is gone.
    expect(screen.getByTestId('client-token-section')).toHaveTextContent('Clicks: 0');
  });

  it('does not remount EditTokenSettings, but forwards the updated sectionResetKey, when it changes', async () => {
    const user = userEvent.setup();
    const {rerender} = render(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: []}}
        onFieldChange={onFieldChange}
        sectionResetKey={0}
      />,
    );

    await user.click(screen.getByTestId('user-token-section-bump'));
    expect(screen.getByTestId('user-token-section')).toHaveTextContent('Clicks: 1');

    rerender(
      <EditTokenSettingsTabs
        application={application}
        editedApp={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: []}}
        onFieldChange={onFieldChange}
        sectionResetKey={1}
      />,
    );

    // EditTokenSettings resets its own form in place on a new key, so it must not remount here
    // (the click count survives) while still receiving the updated key as a prop.
    expect(screen.getByTestId('user-token-section')).toHaveTextContent('Clicks: 1, Key: 1');
  });
});
