// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {ConnectionInstance} from '../../models/connection';
import ConnectionsList from '../ConnectionsList';

const navigateMock = vi.fn();
const useConnectionsMock = vi.hoisted(() => vi.fn());

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigateMock,
}));

vi.mock('../../api/useConnections', () => ({
  default: useConnectionsMock,
}));

const useGetNotificationConfigMock = vi.hoisted(() => vi.fn());
vi.mock('../../api/useGetNotificationConfig', () => ({
  default: useGetNotificationConfigMock,
}));

const EMPTY_STATE_TITLE = 'No connections match your filters';

const OIDC_FEDERATION: ConnectionInstance = {
  id: 'c1',
  name: 'Corp OIDC',
  type: 'oidc',
  categories: ['identity-provider'],
};

const TRUSTED_ISSUER: ConnectionInstance = {
  id: 'c2',
  name: 'Acme Issuer',
  type: 'oidc',
  categories: ['identity-provider'],
  idJagEnabled: true,
};

function mockConnections(connections: ConnectionInstance[]): void {
  useConnectionsMock.mockReturnValue({data: {connections}, isLoading: false, error: null});
}

/** Labels of the rendered category filter chips, excluding the `all` chip. */
function chipLabels(): string[] {
  return within(screen.getByTestId('connection-category-filters'))
    .getAllByText(/.+/)
    .map((node) => node.textContent ?? '')
    .filter((label) => label !== 'All');
}

/** Clicks a category filter chip by its rendered label. */
function selectCategory(label: string): void {
  fireEvent.click(within(screen.getByTestId('connection-category-filters')).getByText(label));
}

describe('ConnectionsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockConnections([]);
    useGetNotificationConfigMock.mockReturnValue({data: undefined});
  });

  it('hides the category chips that no connection card belongs to', () => {
    render(<ConnectionsList />);

    expect(chipLabels()).toEqual(['Social login', 'SMS']);
  });

  it('shows the enterprise and custom chips once an OIDC connection exists', () => {
    mockConnections([OIDC_FEDERATION]);
    render(<ConnectionsList />);

    expect(chipLabels()).toEqual(['Social login', 'Enterprise', 'SMS', 'Custom']);

    selectCategory('Enterprise');

    expect(screen.getByTestId('connection-card-oidc:c1')).toBeInTheDocument();
    expect(screen.queryByText(EMPTY_STATE_TITLE)).not.toBeInTheDocument();
  });

  it('shows the trusted token issuer chip once a trusted issuer exists', () => {
    mockConnections([TRUSTED_ISSUER]);
    render(<ConnectionsList />);

    expect(chipLabels()).toEqual(['Social login', 'SMS', 'Trusted token issuer', 'Custom']);

    selectCategory('Trusted token issuer');

    expect(screen.getByTestId('connection-card-trusted-idp:c2')).toBeInTheDocument();
    expect(screen.queryByText(EMPTY_STATE_TITLE)).not.toBeInTheDocument();
  });

  it('never renders a chip that filters the grid down to nothing', () => {
    mockConnections([OIDC_FEDERATION, TRUSTED_ISSUER]);
    render(<ConnectionsList />);

    for (const label of chipLabels()) {
      selectCategory(label);
      expect(screen.queryByText(EMPTY_STATE_TITLE), `${label} filter is a dead end`).not.toBeInTheDocument();
    }
  });

  it('falls back to all connections when the selected chip is no longer available', () => {
    mockConnections([OIDC_FEDERATION]);
    const {rerender} = render(<ConnectionsList />);

    selectCategory('Enterprise');
    mockConnections([]);
    rerender(<ConnectionsList />);

    expect(chipLabels()).toEqual(['Social login', 'SMS']);
    expect(screen.getByTestId('connection-card-google')).toBeInTheDocument();
    expect(screen.queryByText(EMPTY_STATE_TITLE)).not.toBeInTheDocument();
  });

  it('keeps all selected when a removed category becomes available again', () => {
    mockConnections([OIDC_FEDERATION]);
    const {rerender} = render(<ConnectionsList />);

    selectCategory('Enterprise');
    mockConnections([]);
    rerender(<ConnectionsList />);
    mockConnections([OIDC_FEDERATION]);
    rerender(<ConnectionsList />);

    expect(screen.getByTestId('connection-card-google')).toBeInTheDocument();
    expect(screen.getByTestId('connection-add-custom-card')).toBeInTheDocument();
  });

  it('shows the branded social login tiles under the social login filter', () => {
    render(<ConnectionsList />);

    selectCategory('Social login');

    expect(screen.getByTestId('connection-card-google')).toBeInTheDocument();
    expect(screen.getByTestId('connection-card-github')).toBeInTheDocument();
  });

  it('shows the add-custom card with no filters applied', () => {
    render(<ConnectionsList />);

    expect(screen.getByTestId('connection-add-custom-card')).toBeInTheDocument();
  });

  it('opens the create wizard when the add-custom card is activated', () => {
    render(<ConnectionsList />);

    fireEvent.click(screen.getByTestId('connection-add-custom-card').querySelector('button') as HTMLElement);

    expect(navigateMock).toHaveBeenCalledWith('/connections/create');
  });

  it('shows the empty state with a clear action for a search term that matches nothing', () => {
    render(<ConnectionsList />);

    fireEvent.change(screen.getByPlaceholderText('Search connections'), {target: {value: 'zzz'}});

    expect(screen.getByText(EMPTY_STATE_TITLE)).toBeInTheDocument();
    expect(screen.getAllByText('Clear filters').length).toBeGreaterThan(0);
  });

  it('shows the Default chip on the default email sender card', () => {
    mockConnections([
      {id: 'sm1', name: 'Corp SMTP', type: 'email-smtp', categories: ['email-provider']},
      {id: 'sm2', name: 'Backup SMTP', type: 'email-smtp', categories: ['email-provider']},
    ]);
    useGetNotificationConfigMock.mockReturnValue({
      data: {readOnly: {}, writable: {defaultEmailSenderId: 'sm2'}, merged: {defaultEmailSenderId: 'sm2'}},
    });

    render(<ConnectionsList />);

    expect(within(screen.getByTestId('connection-card-email-smtp:sm2')).getByText('Default')).toBeInTheDocument();
    expect(within(screen.getByTestId('connection-card-email-smtp:sm1')).queryByText('Default')).not.toBeInTheDocument();
  });

  it('reports a notification section that fails to load', () => {
    mockConnections([{id: 'sm1', name: 'Corp SMTP', type: 'email-smtp', categories: ['email-provider']}]);
    useGetNotificationConfigMock.mockReturnValue({data: undefined, error: new Error('boom'), refetch: vi.fn()});

    render(<ConnectionsList />);

    expect(screen.getByText('Failed to load the default email sender.')).toBeInTheDocument();
    expect(within(screen.getByTestId('connection-card-email-smtp:sm1')).queryByText('Default')).not.toBeInTheDocument();
  });
});
