// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useGetFlows} from '@thunderid/configure-flows';
import {MemoryRouter} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Application} from '../../../../models/application';
import {OAuth2GrantTypes} from '../../../../models/oauth';
import CibaFlowSection from '../CibaFlowSection';

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
  QueryErrorNotice: ({error, onRetry = undefined}: {error: Error; onRetry?: () => void}) => (
    <div>
      <p role="alert">{error.message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  ),
}));

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({t: (_key: string, fallback: string) => fallback}),
}));

describe('CibaFlowSection', () => {
  const onFieldChange = vi.fn();
  const flows = [
    {id: 'ciba-flow-email', name: 'CIBA Email Flow', handle: 'ciba-email-flow'},
    {id: 'ciba-flow-sms', name: 'CIBA SMS Flow', handle: 'ciba-sms-flow'},
  ];
  const application = (overrides: Partial<Application> = {}): Application =>
    ({
      id: 'app-1',
      name: 'Example app',
      inboundAuthConfig: [{type: 'oauth2', config: {grantTypes: [OAuth2GrantTypes.CIBA]}}],
      ...overrides,
    }) as Application;

  const renderSection = (app = application(), editedApp: Partial<Application> = {}) =>
    render(
      <MemoryRouter>
        <CibaFlowSection application={app} editedApp={editedApp} onFieldChange={onFieldChange} />
      </MemoryRouter>,
    );

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGetFlows).mockReturnValue({data: {flows}, isLoading: false} as ReturnType<typeof useGetFlows>);
  });

  it('does not render when the CIBA grant is not enabled', () => {
    renderSection(application({inboundAuthConfig: []}));

    expect(screen.queryByText('CIBA flow')).not.toBeInTheDocument();
    expect(useGetFlows).toHaveBeenCalledWith({flowType: 'CIBA', limit: 100, fetchAllPages: true});
  });

  it('shows the query error and retries loading the flow list', async () => {
    const refetch = vi.fn();
    vi.mocked(useGetFlows).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to load flows'),
      refetch,
    } as unknown as ReturnType<typeof useGetFlows>);

    renderSection();

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load flows');
    expect(screen.queryByPlaceholderText('Select a CIBA flow')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Retry'}));
    expect(refetch).toHaveBeenCalledOnce();
  });

  it('uses the query error and retry handler passed from the agent page', async () => {
    const retry = vi.fn();
    render(
      <MemoryRouter>
        <CibaFlowSection
          application={application()}
          editedApp={{}}
          onFieldChange={onFieldChange}
          cibaFlowQueryError={new Error('Agent flow query failed')}
          onRetryCibaFlowQuery={retry}
        />
      </MemoryRouter>,
    );

    expect(screen.getByRole('alert')).toHaveTextContent('Agent flow query failed');
    await userEvent.click(screen.getByRole('button', {name: 'Retry'}));
    expect(retry).toHaveBeenCalledOnce();
  });

  it('renders the selector with the saved selection and says what happens without one', () => {
    renderSection(application({cibaFlowId: 'ciba-flow-email'}));

    expect(screen.getByPlaceholderText('Select a CIBA flow')).toHaveValue('CIBA Email Flow');
    expect(screen.getByText(/When none is selected, the Authentication flow handles the request/)).toBeInTheDocument();
  });

  it('uses the edited selection instead of the saved selection', () => {
    renderSection(application({cibaFlowId: 'ciba-flow-email'}), {cibaFlowId: 'ciba-flow-sms'});

    expect(screen.getByPlaceholderText('Select a CIBA flow')).toHaveValue('CIBA SMS Flow');
  });

  it('shows loading state while flows are being fetched', () => {
    vi.mocked(useGetFlows).mockReturnValue({data: undefined, isLoading: true} as ReturnType<typeof useGetFlows>);
    renderSection();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('updates the application when a flow is selected', async () => {
    const user = userEvent.setup();
    renderSection();

    await user.click(screen.getByPlaceholderText('Select a CIBA flow'));
    await user.click(await screen.findByText('CIBA SMS Flow'));
    expect(onFieldChange).toHaveBeenLastCalledWith('cibaFlowId', 'ciba-flow-sms');
  });

  it('clears the selected flow, so the Authentication flow handles the request again', async () => {
    const user = userEvent.setup();
    renderSection(application({cibaFlowId: 'ciba-flow-email'}));

    await user.click(screen.getByLabelText('Clear'));
    await waitFor(() => expect(onFieldChange).toHaveBeenCalledWith('cibaFlowId', ''));
  });

  it('disables the selector for a readonly application', () => {
    renderSection(application({isReadOnly: true}));

    expect(screen.getByPlaceholderText('Select a CIBA flow')).toBeDisabled();
  });
});
