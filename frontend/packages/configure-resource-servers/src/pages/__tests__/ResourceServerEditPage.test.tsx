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

import {renderWithProviders, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ResourceServer} from '../../models/resource-server';
import ResourceServerEditPage from '../ResourceServerEditPage';

const mockNavigate = vi.fn();
const mockRefetch = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({resourceServerId: 'rs-1'}),
    useSearchParams: () => [new URLSearchParams(), vi.fn()],
    Link: ({to, children = undefined, ...props}: {to: string; children?: ReactNode; [key: string]: unknown}) => (
      <a
        {...(props as Record<string, unknown>)}
        href={to}
        onClick={(e) => {
          e.preventDefault();
          Promise.resolve(mockNavigate(to)).catch(() => null);
        }}
      >
        {children}
      </a>
    ),
  };
});

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: vi.fn()}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090'}),
    useToast: () => ({showToast: vi.fn()}),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('@thunderid/components', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/components')>();
  return {
    ...actual,
    PageLoadingAnimation: vi.fn(() => <div role="progressbar" />),
    SettingsCard: vi.fn(({children, title}: {children: ReactNode; title?: string}) => (
      <div data-testid="settings-card">
        {title && <span>{title}</span>}
        {children}
      </div>
    )),
    UnsavedChangesBar: vi.fn(
      ({message, onReset, onSave}: {message: string; onReset: () => void; onSave: () => void}) => (
        <div data-testid="unsaved-changes-bar">
          <span>{message}</span>
          <button type="button" onClick={onReset}>
            Discard
          </button>
          <button type="button" onClick={onSave}>
            Save
          </button>
        </div>
      ),
    ),
  };
});

const mockUseGetResourceServer = vi.fn();
const mockUpdateMutate = vi.fn();

vi.mock('../../api/useGetResourceServer', () => ({
  default: () =>
    mockUseGetResourceServer() as {
      data: ResourceServer | undefined;
      isLoading: boolean;
      error: Error | null;
      refetch: () => void;
    },
}));

vi.mock('../../api/useUpdateResourceServer', () => ({
  default: () => ({mutate: mockUpdateMutate, isPending: false}),
}));

vi.mock('../../api/useGetResources', () => ({
  default: () => ({data: {resources: [], totalResults: 0, startIndex: 0, count: 0}, isLoading: false}),
}));

vi.mock('../../api/useGetServerActions', () => ({
  default: () => ({data: {actions: [], totalResults: 0, startIndex: 0, count: 0}, isLoading: false}),
}));

vi.mock('../../components/ResourceServerDeleteDialog', () => ({
  default: () => null,
}));

vi.mock('../../components/resource-tree/ResourceTree', () => ({
  default: () => <div data-testid="resource-tree" />,
}));

vi.mock('../../components/resource-server-detail/AdvancedTab', () => ({
  default: () => <div data-testid="advanced-tab" />,
}));

const mockResourceServer: ResourceServer = {
  id: 'rs-1',
  name: 'Dark Dodos Smash',
  handle: 'dark-dodos',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: '/',
  type: 'API',
};

const readOnlyResourceServer: ResourceServer = {
  ...mockResourceServer,
  isReadOnly: true,
};

describe('ResourceServerEditPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetResourceServer.mockReturnValue({
      data: mockResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });
  });

  it('renders the loading animation when data is loading', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders the error alert when the fetch fails', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Network error')).toBeInTheDocument();
  });

  it('renders the resource server name after successful load', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Dark Dodos Smash')).toBeInTheDocument();
  });

  it('renders the handle chip after successful load', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('dark-dodos')).toBeInTheDocument();
  });

  it('renders the Resources tab as active by default', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByRole('tab', {name: 'Resources', selected: true})).toBeInTheDocument();
  });

  it('renders the ResourceTree in the Resources tab by default', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByTestId('resource-tree')).toBeInTheDocument();
  });

  it('shows the AdvancedTab when the Advanced Settings tab is clicked', async () => {
    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced Settings'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });
  });

  it('renders the Danger Zone card for a non-read-only server', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Danger Zone')).toBeInTheDocument();
  });

  it('renders the delete button inside the Danger Zone', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByRole('button', {name: /Delete resource server/i})).toBeInTheDocument();
  });

  it('does not render the Danger Zone for a read-only server', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: readOnlyResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.queryByText('Danger Zone')).not.toBeInTheDocument();
  });

  it('renders the read-only info alert for a read-only server', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: readOnlyResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText(/This resource is read-only and cannot be modified/i)).toBeInTheDocument();
  });

  it('shows the name text field when the edit icon button is clicked', async () => {
    renderWithProviders(<ResourceServerEditPage />);

    const editButtons = screen.getAllByRole('button');
    const nameEditButton = editButtons.find(
      (btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Dark Dodos Smash'),
    );

    expect(nameEditButton).toBeDefined();
    fireEvent.click(nameEditButton!);

    await waitFor(() => {
      expect(screen.getByDisplayValue('Dark Dodos Smash')).toBeInTheDocument();
    });
  });
});
