// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useGetFlows} from '@thunderid/configure-flows';
import {MemoryRouter} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Application} from '../../../../models/application';
import {OAuth2GrantTypes} from '../../../../models/oauth';
import DeviceGrantFlowSection from '../DeviceGrantFlowSection';

vi.mock('@thunderid/configure-flows', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-flows')>()),
  useGetFlows: vi.fn(),
}));

vi.mock('@thunderid/components', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/components')>()),
  SettingsCard: ({title, description, children}: {title: string; description: string; children: React.ReactNode}) => (
    <section>
      <h2>{title}</h2>
      <p>{description}</p>
      {children}
    </section>
  ),
}));

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({t: (_key: string, fallback: string) => fallback}),
}));

describe('DeviceGrantFlowSection', () => {
  const onFieldChange = vi.fn();
  const flows = [
    {id: 'device-flow-default', name: 'Default Device Flow', handle: 'default-device-grant-flow'},
    {id: 'device-flow-custom', name: 'Custom Device Flow', handle: 'custom-device-grant-flow'},
  ];
  const application = (overrides: Partial<Application> = {}): Application =>
    ({
      id: 'app-1',
      name: 'Example app',
      inboundAuthConfig: [{type: 'oauth2', config: {grantTypes: [OAuth2GrantTypes.DEVICE_CODE]}}],
      ...overrides,
    }) as Application;

  const renderSection = (app = application(), editedApp: Partial<Application> = {}) =>
    render(
      <MemoryRouter>
        <DeviceGrantFlowSection application={app} editedApp={editedApp} onFieldChange={onFieldChange} />
      </MemoryRouter>,
    );

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGetFlows).mockReturnValue({data: {flows}, isLoading: false} as ReturnType<typeof useGetFlows>);
  });

  it('does not render when Device Grant is not enabled', () => {
    renderSection(application({inboundAuthConfig: []}));

    expect(screen.queryByText('Device authorization flow')).not.toBeInTheDocument();
    expect(useGetFlows).toHaveBeenCalledWith({flowType: 'DEVICE_GRANT'});
  });

  it('renders the selector and default flow notice for the saved selection', () => {
    renderSection(application({deviceGrantFlowId: 'device-flow-default'}));

    expect(screen.getByText(/Choose the flow used to authorize devices for this/)).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('The default Device Grant flow is selected');
    expect(screen.getByPlaceholderText('Select a Device Grant flow')).toHaveValue('Default Device Flow');
  });

  it('uses the edited selection instead of the saved selection', () => {
    renderSection(application({deviceGrantFlowId: 'device-flow-default'}), {deviceGrantFlowId: 'device-flow-custom'});

    expect(screen.getByPlaceholderText('Select a Device Grant flow')).toHaveValue('Custom Device Flow');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('shows loading state while flows are being fetched', () => {
    vi.mocked(useGetFlows).mockReturnValue({data: undefined, isLoading: true} as ReturnType<typeof useGetFlows>);
    renderSection();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('updates the application when a flow is selected or cleared', async () => {
    const user = userEvent.setup();
    renderSection();
    const input = screen.getByPlaceholderText('Select a Device Grant flow');

    await user.click(input);
    await user.click(await screen.findByText('Custom Device Flow'));
    expect(onFieldChange).toHaveBeenLastCalledWith('deviceGrantFlowId', 'device-flow-custom');
  });

  it('clears the selected flow', async () => {
    const user = userEvent.setup();
    renderSection(application({deviceGrantFlowId: 'device-flow-default'}));

    await user.click(screen.getByLabelText('Clear'));
    await waitFor(() => expect(onFieldChange).toHaveBeenCalledWith('deviceGrantFlowId', ''));
  });

  it('disables the selector for a readonly application', () => {
    renderSection(application({isReadOnly: true}));

    expect(screen.getByPlaceholderText('Select a Device Grant flow')).toBeDisabled();
  });
});
