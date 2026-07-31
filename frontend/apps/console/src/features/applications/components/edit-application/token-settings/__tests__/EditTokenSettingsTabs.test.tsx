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

import userEvent from '@testing-library/user-event';
import type {Application} from '@thunderid/configure-applications';
import {render, screen} from '@thunderid/test-utils';
import {useState} from 'react';
import {describe, it, expect, vi} from 'vitest';
import EditTokenSettingsTabs from '../EditTokenSettingsTabs';

vi.mock('../ClientAccessTokenSection', () => ({
  default: vi.fn(() => {
    const [clicks, setClicks] = useState(0);
    return (
      <div data-testid="client-token-section">
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
const clientLockMessage = /client credentials grant is enabled/i;
const userLockMessage = /user-facing grant is enabled/i;

describe('EditTokenSettingsTabs', () => {
  const onFieldChange = vi.fn();

  it('renders Application and User sub-tabs', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.getByRole('tab', {name: 'Application'})).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'User'})).toBeInTheDocument();
  });

  it('unlocks the Application tab when client_credentials is granted', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.getByTestId('client-token-section')).toBeInTheDocument();
    expect(screen.queryByText(clientLockMessage)).not.toBeInTheDocument();
  });

  it('freezes the Application tab with a lock notice when client_credentials is not granted', () => {
    render(
      <EditTokenSettingsTabs
        application={application}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: []}}
        onFieldChange={onFieldChange}
      />,
    );

    expect(screen.getByText(clientLockMessage)).toBeInTheDocument();
  });

  it('freezes the User tab with a lock notice when no user-facing grant is present', async () => {
    const user = userEvent.setup();
    render(
      <EditTokenSettingsTabs
        application={application}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={onFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(screen.getByText(userLockMessage)).toBeInTheDocument();
  });

  it('unlocks the User tab when a user-facing grant is present', async () => {
    const user = userEvent.setup();
    render(
      <EditTokenSettingsTabs
        application={application}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: []}}
        onFieldChange={onFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(screen.getByTestId('user-token-section')).toBeInTheDocument();
    expect(screen.queryByText(userLockMessage)).not.toBeInTheDocument();
  });

  it('remounts ClientAccessTokenSection when sectionResetKey changes', async () => {
    const user = userEvent.setup();
    const {rerender} = render(
      <EditTokenSettingsTabs
        application={application}
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
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: []}}
        onFieldChange={onFieldChange}
        sectionResetKey={0}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));
    await user.click(screen.getByTestId('user-token-section-bump'));
    expect(screen.getByTestId('user-token-section')).toHaveTextContent('Clicks: 1');

    rerender(
      <EditTokenSettingsTabs
        application={application}
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
