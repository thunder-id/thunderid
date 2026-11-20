// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, ProjectScopeProvider} from '@thunderid/contexts';
import {renderWithProviders, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {AuthorizationDetailTypeListResponse} from '../../models/authorization-detail-type';
import AuthorizationDetailTypesListPage from '../AuthorizationDetailTypesListPage';

const mockNavigate = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {...actual, useThunderID: () => ({http: {request: mockHttpRequest}})};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: () => 'https://localhost:8090',
      getDocumentationLink: () => undefined,
      config: {brand: {product_name: 'ThunderID'}},
    }),
    useToast: () => ({showToast: vi.fn()}),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

const page: AuthorizationDetailTypeListResponse = {
  totalResults: 1,
  startIndex: 0,
  count: 1,
  types: [
    {
      id: 'type-1',
      type: 'payment_initiation',
      name: 'Payment initiation',
      version: 1,
      schema: {type: 'object', properties: {}},
      reuse: true,
    },
  ],
};

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  isLoading: false,
};

describe('AuthorizationDetailTypesListPage', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockNavigate.mockReset();
    mockNavigate.mockResolvedValue(undefined);
    mockHttpRequest.mockResolvedValue({data: page});
  });

  it('lists the types from the top-level endpoint', async () => {
    renderWithProviders(<AuthorizationDetailTypesListPage />);

    expect(screen.getByRole('heading', {name: 'Intents'})).toBeInTheDocument();
    expect(
      screen.getByText(
        'Define fine-grained, structured permissions that clients can ask users to approve through Rich Authorization Requests (RAR).',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Learn more/}), 'no documentation link is configured').toBeNull();
    expect(await screen.findByText('Payment initiation')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/authorization-detail-types?limit=10&offset=0',
      method: 'GET',
    });
  });

  it('opens the create page to add a type', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypesListPage />);

    await user.click(screen.getByRole('button', {name: 'Add intent'}));
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types/create');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('hides the add button when the admin may not create intents', () => {
    renderWithProviders(
      <CapabilitiesProvider capabilities={viewOnly}>
        <AuthorizationDetailTypesListPage />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByRole('button', {name: 'Add intent'})).not.toBeInTheDocument();
  });

  it("lists only the selected project's types inside a project", async () => {
    renderWithProviders(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <AuthorizationDetailTypesListPage createInProjectsOnly />
      </ProjectScopeProvider>,
    );

    expect(await screen.findByText('Payment initiation')).toBeInTheDocument();
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/authorization-detail-types?limit=10&offset=0&projectId=project-1',
      method: 'GET',
    });
    expect(screen.getByRole('button', {name: 'Add intent'})).toBeInTheDocument();
  });

  it('offers no add button outside a project when intents are created in projects only', async () => {
    renderWithProviders(<AuthorizationDetailTypesListPage createInProjectsOnly />);

    expect(await screen.findByText('Payment initiation')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add intent'})).not.toBeInTheDocument();
  });

  it('sets the document title', async () => {
    renderWithProviders(<AuthorizationDetailTypesListPage />);

    await waitFor(() => expect(document.title).toBe('Intents | ThunderID'));
  });

  it('opens the edit page of a listed type', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypesListPage />);

    await user.click(await screen.findByText('Payment initiation'));
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types/type-1');
  });
});
