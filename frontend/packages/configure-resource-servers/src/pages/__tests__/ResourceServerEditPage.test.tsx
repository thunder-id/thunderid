// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import * as componentsModule from '@thunderid/components';
import {OrganizationAccessSettings} from '@thunderid/configure-resource-sharing';
import {CapabilitiesProvider, HiddenResourcesProvider} from '@thunderid/contexts';
import {renderWithProviders, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {ResourceServer} from '../../models/resource-server';
import ResourceServerEditPage from '../ResourceServerEditPage';

const mockNavigate = vi.fn();
const mockRefetch = vi.fn();
const mockUseParams = vi.hoisted(() => vi.fn(() => ({resourceServerId: 'rs-1'})));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: mockUseParams,
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

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    useThunderID: () => ({http: {request: vi.fn()}}),
  };
});

const mockUseProjectScope = vi.fn<() => {projectId?: string}>(() => ({}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090', config: {brand: {product_name: 'ThunderID'}}}),
    useProjectScope: () => mockUseProjectScope(),
    useToast: () => ({showToast: vi.fn()}),
  };
});

const mockOrganizationAccess = vi.hoisted(() => ({
  hasChanges: false,
  save: vi.fn(),
  reset: vi.fn(),
  isSaving: false,
  saveError: null as Error | null,
}));

const mockUseOrganizationAccess = vi.hoisted(() => vi.fn());

vi.mock('@thunderid/configure-resource-sharing', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-resource-sharing')>()),
  useOrganizationAccess: mockUseOrganizationAccess,
  OrganizationAccessSettings: vi.fn(() => <div data-testid="organization-access-settings" />),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('@thunderid/components', {spy: true});

vi.mocked(componentsModule.PageLoadingAnimation).mockImplementation(() => <div role="progressbar" />);
vi.mocked(componentsModule.SettingsCard).mockImplementation(
  ({children, title}: {children: ReactNode; title?: string}) => (
    <div data-testid="settings-card">
      {title && <span>{title}</span>}
      {children}
    </div>
  ),
);
vi.mocked(componentsModule.UnsavedChangesBar).mockImplementation(
  ({
    message,
    resetLabel,
    saveLabel,
    saveDisabled = false,
    isSaving = false,
    resetDisabled = false,
    error = undefined,
    onReset,
    onSave,
  }: {
    message: string;
    resetLabel: string;
    saveLabel: string;
    saveDisabled?: boolean;
    isSaving?: boolean;
    resetDisabled?: boolean;
    error?: string;
    onReset: () => void;
    onSave: () => void;
  }) => (
    <div data-testid="unsaved-changes-bar">
      {error && <span>{error}</span>}
      <span>{message}</span>
      <button type="button" onClick={onReset} disabled={resetDisabled}>
        {resetLabel}
      </button>
      <button type="button" onClick={onSave} disabled={saveDisabled || isSaving}>
        {saveLabel}
      </button>
    </div>
  ),
);

const mockUseGetResourceServer = vi.fn();
const mockUpdateMutate = vi.fn();
const mockUpdateReset = vi.fn();
interface UpdateResourceServerMutationMock {
  mutate: typeof mockUpdateMutate;
  isPending: boolean;
  isError: boolean;
  error: Error | null;
  reset: typeof mockUpdateReset;
}

const mockUseUpdateResourceServer = vi.fn(
  (): UpdateResourceServerMutationMock => ({
    mutate: mockUpdateMutate,
    isPending: false,
    isError: false,
    error: null,
    reset: mockUpdateReset,
  }),
);

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
  default: () => mockUseUpdateResourceServer(),
}));

const mockUseGetDefaultResourceServer = vi.fn();

vi.mock('../../api/useGetDefaultResourceServer', () => ({
  default: () => mockUseGetDefaultResourceServer() as {data: unknown},
}));

vi.mock('../../components/SetDefaultResourceServerDialog', () => ({
  default: () => null,
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
  default: ({
    identifier,
    isIdentifierInvalid = false,
    onIdentifierChange,
  }: {
    identifier: string;
    isIdentifierInvalid?: boolean;
    onIdentifierChange: (value: string) => void;
  }) => (
    <div data-testid="advanced-tab">
      <input
        aria-label="Identifier"
        aria-invalid={isIdentifierInvalid}
        value={identifier}
        onChange={(e) => onIdentifierChange(e.target.value)}
      />
    </div>
  ),
}));

const mockResourceServer: ResourceServer = {
  id: 'rs-1',
  name: 'Dark Dodos Smash',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: '/',
  type: 'API',
};

const readOnlyResourceServer: ResourceServer = {
  ...mockResourceServer,
  isReadOnly: true,
};

const mockMcpResourceServer: ResourceServer = {
  ...mockResourceServer,
  type: 'MCP',
};

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  isLoading: false,
};

describe('ResourceServerEditPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({resourceServerId: 'rs-1'});
    mockUseGetResourceServer.mockReturnValue({
      data: mockResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });
    mockUseGetDefaultResourceServer.mockReturnValue({
      data: {readOnly: {}, writable: {}, merged: {}},
    });
    mockUseUpdateResourceServer.mockReturnValue({
      mutate: mockUpdateMutate,
      isPending: false,
      isError: false,
      error: null,
      reset: mockUpdateReset,
    });
    mockOrganizationAccess.hasChanges = false;
    mockOrganizationAccess.isSaving = false;
    mockOrganizationAccess.saveError = null;
    mockOrganizationAccess.save.mockReset().mockResolvedValue(undefined);
    mockOrganizationAccess.reset.mockReset();
    mockUseOrganizationAccess.mockReset().mockReturnValue(mockOrganizationAccess);
  });

  describe('a hidden resource server', () => {
    it('returns to the list when the resource server is hidden by its identifier', async () => {
      renderWithProviders(
        <HiddenResourcesProvider hidden={{resourceServerIdentifiers: ['https://api.example.com']}}>
          <ResourceServerEditPage />
        </HiddenResourcesProvider>,
      );

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/resource-servers', {replace: true});
      });
    });

    it('shows the resource server when another identifier is hidden', () => {
      renderWithProviders(
        <HiddenResourcesProvider hidden={{resourceServerIdentifiers: ['var:DATA_PLANE_URL']}}>
          <ResourceServerEditPage />
        </HiddenResourcesProvider>,
      );

      expect(screen.getByText(mockResourceServer.name)).toBeInTheDocument();
      expect(mockNavigate).not.toHaveBeenCalled();
    });
  });

  describe('inside a project', () => {
    afterEach(() => {
      mockUseProjectScope.mockReturnValue({});
    });

    it('returns to the list for a resource server of another project', async () => {
      mockUseProjectScope.mockReturnValue({projectId: 'project-1'});
      mockUseGetResourceServer.mockReturnValue({
        data: {...mockResourceServer, projectId: 'project-2'},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<ResourceServerEditPage />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/resource-servers', {replace: true});
      });
    });

    it('shows a resource server of the selected project', () => {
      mockUseProjectScope.mockReturnValue({projectId: 'project-1'});
      mockUseGetResourceServer.mockReturnValue({
        data: {...mockResourceServer, projectId: 'project-1'},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<ResourceServerEditPage />);

      expect(screen.getByText(mockResourceServer.name)).toBeInTheDocument();
      expect(mockNavigate).not.toHaveBeenCalled();
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

  it('renders the resolved catalog message in the error state, never the raw server text, when the fetch fails', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Failed to load resource server')).toBeInTheDocument();
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.queryByText('Network error')).not.toBeInTheDocument();
  });

  it('retries the fetch when Refresh is clicked in the error state', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('button', {name: /Refresh/i}));

    expect(mockRefetch).toHaveBeenCalled();
  });

  it('navigates back to the resource servers list from the error state', async () => {
    mockUseGetResourceServer.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('button', {name: /Back to resource servers/i}));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/resource-servers');
    });
  });

  it('renders a distinct not-found message, not the generic error state, when the fetch succeeds with no data', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Resource server not found.')).toBeInTheDocument();
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
  });

  it('renders the resource server name after successful load', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Dark Dodos Smash')).toBeInTheDocument();
  });

  it('sets the document title to the resource server name', async () => {
    renderWithProviders(<ResourceServerEditPage />);

    await waitFor(() => expect(document.title).toBe('Dark Dodos Smash | ThunderID'));
  });

  it('does not render a resource server handle chip', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.queryByText('dark-dodos')).not.toBeInTheDocument();
  });

  it('renders the Resources tab as active by default', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByRole('tab', {name: 'Resources', selected: true})).toBeInTheDocument();
  });

  it('renders the ResourceTree in the Resources tab by default', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByTestId('resource-tree')).toBeInTheDocument();
  });

  it('shows the AdvancedTab when the Advanced tab is clicked', async () => {
    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });
  });

  it('renders the Danger Zone card for a non-read-only server', () => {
    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    expect(screen.getByText('Danger zone')).toBeInTheDocument();
  });

  it('renders the delete button inside the Danger Zone', () => {
    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    expect(screen.getByRole('button', {name: /Delete resource server/i})).toBeInTheDocument();
  });

  it('omits the Danger Zone and disables Save when the admin may only view resource servers', async () => {
    renderWithProviders(
      <CapabilitiesProvider capabilities={viewOnly}>
        <ResourceServerEditPage />
      </CapabilitiesProvider>,
    );

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    expect(screen.queryByText('Danger zone')).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });
  });

  it('does not render the Danger Zone for a read-only server', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: readOnlyResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    expect(screen.queryByText('Danger zone')).not.toBeInTheDocument();
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

  it('shows the unsaved changes bar when the identifier is edited in the Advanced tab', async () => {
    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByTestId('unsaved-changes-bar')).toBeInTheDocument();
    });
  });

  it('includes the edited identifier when Save is clicked from the unsaved changes bar', async () => {
    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByTestId('unsaved-changes-bar')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: 'Save'}));

    expect(mockUpdateMutate).toHaveBeenCalledWith(
      {
        id: 'rs-1',
        data: {
          name: 'Dark Dodos Smash',
          description: null,
          identifier: 'https://new-api.example.com',
          ouId: 'ou-1',
          authorizationEngine: {
            type: 'rbac',
          },
        },
      },
      expect.any(Object),
    );
  });

  it('does not save when the identifier is cleared', async () => {
    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: '   '}});

    await waitFor(() => {
      expect(screen.getByTestId('unsaved-changes-bar')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: 'Save'}));

    expect(mockUpdateMutate).not.toHaveBeenCalled();
  });

  it('renders the MCP-specific Danger Zone title for an MCP server', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: mockMcpResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    expect(screen.getByRole('heading', {name: 'Delete MCP server'})).toBeInTheDocument();
  });

  it('renders the MCP-specific Danger Zone description for an MCP server', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: mockMcpResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    expect(
      screen.getByText('Permanently delete this MCP server and all associated data. This action cannot be undone.'),
    ).toBeInTheDocument();
  });

  it('renders the MCP-specific delete button label for an MCP server', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: mockMcpResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    expect(screen.getByRole('button', {name: 'Delete MCP server'})).toBeInTheDocument();
  });

  it('shows the Set as default button when the server is not the default', () => {
    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByRole('button', {name: 'Set as default'})).toBeInTheDocument();
    expect(screen.queryByText('Default resource server')).not.toBeInTheDocument();
  });

  it('shows the Default resource server badge when the server is the default', () => {
    mockUseGetDefaultResourceServer.mockReturnValue({
      data: {readOnly: {}, writable: {}, merged: {resourceServerId: 'rs-1'}},
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Default resource server')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Set as default'})).not.toBeInTheDocument();
  });

  it('renders neither the badge nor the action while the default config is loading', () => {
    mockUseGetDefaultResourceServer.mockReturnValue({data: undefined, isLoading: true, error: null});

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.queryByRole('button', {name: 'Set as default'})).not.toBeInTheDocument();
    expect(screen.queryByText('Default resource server')).not.toBeInTheDocument();
  });

  it('does not offer Set as default when the default is locked by declarative config', () => {
    mockUseGetDefaultResourceServer.mockReturnValue({
      data: {readOnly: {resourceServerId: 'rs-9'}, writable: {}, merged: {resourceServerId: 'rs-9'}},
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.queryByRole('button', {name: 'Set as default'})).not.toBeInTheDocument();
    expect(screen.queryByText('Default resource server')).not.toBeInTheDocument();
  });

  it('shows the badge but no action for a locked default server', () => {
    mockUseGetDefaultResourceServer.mockReturnValue({
      data: {readOnly: {resourceServerId: 'rs-1'}, writable: {}, merged: {resourceServerId: 'rs-1'}},
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Default resource server')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Set as default'})).not.toBeInTheDocument();
  });

  it('does not offer Set as default for an MCP server, which is not an eligible type', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: mockMcpResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.queryByRole('button', {name: 'Set as default'})).not.toBeInTheDocument();
  });

  // The backend accepts an MCP default, so this state is reachable through the API or declarative
  // config. The console must report it rather than hide it.
  it('still shows the badge for an MCP server that is already the default', () => {
    mockUseGetResourceServer.mockReturnValue({
      data: mockMcpResourceServer,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });
    mockUseGetDefaultResourceServer.mockReturnValue({
      data: {readOnly: {}, writable: {}, merged: {resourceServerId: 'rs-1'}},
    });

    renderWithProviders(<ResourceServerEditPage />);

    expect(screen.getByText('Default resource server')).toBeInTheDocument();
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

  it('shows the resolved catalog message inline, never the raw server text, when a save fails', async () => {
    mockUseUpdateResourceServer.mockReturnValue({
      mutate: mockUpdateMutate,
      isPending: false,
      isError: true,
      error: new Error('raw backend save failure detail'),
      reset: mockUpdateReset,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByText('Failed to save changes.')).toBeInTheDocument();
    });
    expect(screen.queryByText('raw backend save failure detail')).not.toBeInTheDocument();
  });

  it('resets the save error as soon as an edited field is changed', async () => {
    mockUseUpdateResourceServer.mockReturnValue({
      mutate: mockUpdateMutate,
      isPending: false,
      isError: true,
      error: new Error('save failed'),
      reset: mockUpdateReset,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'https://new-api.example.com'}});

    expect(mockUpdateReset).toHaveBeenCalled();
  });

  it('does not reset a pending update mutation when a field changes', async () => {
    mockUseUpdateResourceServer.mockReturnValue({
      mutate: mockUpdateMutate,
      isPending: true,
      isError: false,
      error: null,
      reset: mockUpdateReset,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'https://new-api.example.com'}});

    expect(mockUpdateReset).not.toHaveBeenCalled();
  });

  it('resets a failed update mutation when the unsaved changes bar is reset', async () => {
    mockUseUpdateResourceServer.mockReturnValue({
      mutate: mockUpdateMutate,
      isPending: false,
      isError: true,
      error: new Error('save failed'),
      reset: mockUpdateReset,
    });

    renderWithProviders(<ResourceServerEditPage />);

    fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

    await waitFor(() => {
      expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByTestId('unsaved-changes-bar')).toBeInTheDocument();
    });

    mockUpdateReset.mockClear();
    fireEvent.click(screen.getByRole('button', {name: 'Reset'}));

    expect(mockUpdateReset).toHaveBeenCalled();
  });

  describe('MCP server identifier validation', () => {
    const renderMcpServer = async (identifier = 'https://mcp.example.com/mcp'): Promise<void> => {
      mockUseGetResourceServer.mockReturnValue({
        data: {...mockMcpResourceServer, identifier},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });

      renderWithProviders(<ResourceServerEditPage />);

      fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

      await waitFor(() => {
        expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
      });
    };

    it.each([
      'my-mcp-server',
      '/mcp',
      'mcp.example.com',
      'localhost:8000/mcp',
      'mcp.example.com:8443/mcp',
      'urn:example:mcp',
      'https://mcp.example.com/#x',
    ])('should flag the edited identifier %s and block the save', async (value) => {
      await renderMcpServer();

      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value}});

      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'true');
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();

      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      expect(mockUpdateMutate).not.toHaveBeenCalled();
    });

    it('should save a valid edited identifier, trimmed', async () => {
      await renderMcpServer();

      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: '  https://new-mcp.example.com/mcp  '}});

      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'false');
      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      expect(mockUpdateMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          data: expect.objectContaining({identifier: 'https://new-mcp.example.com/mcp'}) as unknown,
        }),
        expect.any(Object),
      );
    });

    it('should clear the error once the identifier is corrected', async () => {
      await renderMcpServer();

      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'my-mcp-server'}});
      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'true');

      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'http://localhost:8000/mcp'}});

      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'false');
      expect(screen.getByRole('button', {name: 'Save'})).toBeEnabled();
    });

    it('should not flag a cleared identifier, which the required check handles', async () => {
      await renderMcpServer();

      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: '   '}});

      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'false');
      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      expect(mockUpdateMutate).not.toHaveBeenCalled();
    });

    it('should not flag a saved identifier that predates the rule while other fields are edited', async () => {
      await renderMcpServer('legacy-mcp');

      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'false');

      const nameEditButton = screen
        .getAllByRole('button')
        .find((btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Dark Dodos Smash'));
      fireEvent.click(nameEditButton!);
      const nameInput = await screen.findByDisplayValue('Dark Dodos Smash');
      fireEvent.change(nameInput, {target: {value: 'Renamed MCP server'}});
      fireEvent.keyDown(nameInput, {key: 'Enter'});

      expect(screen.getByRole('button', {name: 'Save'})).toBeEnabled();
      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      expect(mockUpdateMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          data: expect.objectContaining({name: 'Renamed MCP server', identifier: 'legacy-mcp'}) as unknown,
        }),
        expect.any(Object),
      );
    });

    it('should flag a saved identifier that predates the rule once it is changed to another invalid value', async () => {
      await renderMcpServer('legacy-mcp');

      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'legacy-mcp-2'}});

      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'true');
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('should not flag a non-URI identifier on an API resource server', async () => {
      renderWithProviders(<ResourceServerEditPage />);

      fireEvent.click(screen.getByRole('tab', {name: 'Advanced'}));

      await waitFor(() => {
        expect(screen.getByTestId('advanced-tab')).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'my-api'}});

      expect(screen.getByLabelText('Identifier')).toHaveAttribute('aria-invalid', 'false');
      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      expect(mockUpdateMutate).toHaveBeenCalledWith(
        expect.objectContaining({data: expect.objectContaining({identifier: 'my-api'}) as unknown}),
        expect.any(Object),
      );
    });
  });

  describe('Organization Access tab', () => {
    const openAccessTab = (): void => {
      fireEvent.click(screen.getByRole('tab', {name: 'Organization Access'}));
    };

    const renameServer = async (name: string): Promise<void> => {
      const nameEditButton = screen
        .getAllByRole('button')
        .find((btn) => btn.querySelector('svg') && btn.closest('div')?.textContent?.includes('Dark Dodos Smash'));
      fireEvent.click(nameEditButton!);
      const nameInput = await screen.findByDisplayValue('Dark Dodos Smash');
      fireEvent.change(nameInput, {target: {value: name}});
      fireEvent.keyDown(nameInput, {key: 'Enter'});
    };

    it('places the tab between Resources and Advanced', () => {
      renderWithProviders(<ResourceServerEditPage />);

      expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
        'Resources',
        'Organization Access',
        'Advanced',
      ]);
    });

    it('links each tab to its panel', () => {
      renderWithProviders(<ResourceServerEditPage />);
      openAccessTab();

      const tab = screen.getByRole('tab', {name: 'Organization Access'});
      const panel = screen.getByRole('tabpanel', {name: 'Organization Access'});
      expect(tab).toHaveAttribute('aria-controls', panel.id);
      expect(panel).toHaveAttribute('aria-labelledby', tab.id);
    });

    it('loads the sharing policies only once the tab is opened', async () => {
      renderWithProviders(<ResourceServerEditPage />);

      expect(mockUseOrganizationAccess).toHaveBeenLastCalledWith({
        resourcePath: 'resource-servers/rs-1',
        enabled: false,
        wordingNamespace: 'resourceServers',
      });

      openAccessTab();

      await waitFor(() => {
        expect(mockUseOrganizationAccess).toHaveBeenLastCalledWith(expect.objectContaining({enabled: true}));
      });
    });

    it('starts afresh when the page moves to another resource server', async () => {
      const {rerender} = renderWithProviders(<ResourceServerEditPage />);
      openAccessTab();
      await waitFor(() => {
        expect(mockUseOrganizationAccess).toHaveBeenLastCalledWith(expect.objectContaining({enabled: true}));
      });

      mockUseParams.mockReturnValue({resourceServerId: 'rs-2'});
      rerender(<ResourceServerEditPage />);

      expect(mockUseOrganizationAccess).toHaveBeenLastCalledWith({
        resourcePath: 'resource-servers/rs-2',
        enabled: false,
        wordingNamespace: 'resourceServers',
      });
      expect(screen.getByRole('tab', {name: 'Resources'})).toHaveAttribute('aria-selected', 'true');
    });

    it('renders the settings for the resource server', async () => {
      renderWithProviders(<ResourceServerEditPage />);

      openAccessTab();

      expect(await screen.findByTestId('organization-access-settings')).toBeInTheDocument();
      expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual({
        access: mockOrganizationAccess,
        ownerOuId: 'ou-1',
        resourceName: 'Dark Dodos Smash',
        readOnly: false,
        errorNamespace: 'resourceServers',
      });
    });

    it('makes the settings read-only for a read-only resource server', async () => {
      mockUseGetResourceServer.mockReturnValue({
        data: readOnlyResourceServer,
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      });
      renderWithProviders(<ResourceServerEditPage />);

      openAccessTab();

      await screen.findByTestId('organization-access-settings');
      expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0]).toEqual(
        expect.objectContaining({readOnly: true}),
      );
    });

    it('saves only the organization access when nothing else changed', async () => {
      mockOrganizationAccess.hasChanges = true;
      renderWithProviders(<ResourceServerEditPage />);

      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => expect(mockOrganizationAccess.save).toHaveBeenCalled());
      expect(mockUpdateMutate).not.toHaveBeenCalled();
    });

    it('saves the organization access after the resource server update succeeds', async () => {
      mockOrganizationAccess.hasChanges = true;
      mockUpdateMutate.mockImplementation((_vars: unknown, options: {onSuccess: () => void}) => {
        expect(mockOrganizationAccess.save).not.toHaveBeenCalled();
        options.onSuccess();
      });
      renderWithProviders(<ResourceServerEditPage />);

      await renameServer('Renamed server');
      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      expect(mockUpdateMutate).toHaveBeenCalled();
      await waitFor(() => expect(mockOrganizationAccess.save).toHaveBeenCalled());
    });

    it('does not save the organization access when the resource server update fails', async () => {
      mockOrganizationAccess.hasChanges = true;
      mockUpdateMutate.mockImplementation((_vars: unknown, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Request failed'));
      });
      renderWithProviders(<ResourceServerEditPage />);

      await renameServer('Renamed server');
      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      expect(mockUpdateMutate).toHaveBeenCalled();
      expect(mockOrganizationAccess.save).not.toHaveBeenCalled();
    });

    it('locks the settings until the whole save finishes', async () => {
      mockOrganizationAccess.hasChanges = true;
      let finishAccessSave: () => void = () => undefined;
      mockOrganizationAccess.save.mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            finishAccessSave = resolve;
          }),
      );
      let finishUpdate: () => void = () => undefined;
      mockUpdateMutate.mockImplementation((_vars: unknown, options: {onSuccess: () => void}) => {
        finishUpdate = options.onSuccess;
      });
      const lastReadOnly = (): unknown => vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0].readOnly;
      renderWithProviders(<ResourceServerEditPage />);
      openAccessTab();
      await screen.findByTestId('organization-access-settings');
      await renameServer('Renamed server');

      fireEvent.click(screen.getByRole('button', {name: 'Save'}));
      await waitFor(() => expect(lastReadOnly()).toBe(true));

      finishUpdate();
      await waitFor(() => expect(mockOrganizationAccess.save).toHaveBeenCalled());
      expect(lastReadOnly()).toBe(true);
      // The access save resolves only after its refetch and draft clear, so Save and Reset wait for all of it.
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Reset'})).toBeDisabled();

      finishAccessSave();
      await waitFor(() => expect(lastReadOnly()).toBe(false));
      expect(mockOrganizationAccess.save).toHaveBeenCalledTimes(1);
      expect(mockOrganizationAccess.reset).not.toHaveBeenCalled();
    });

    it('unlocks the settings when the resource server update fails', async () => {
      mockOrganizationAccess.hasChanges = true;
      mockUpdateMutate.mockImplementation((_vars: unknown, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Request failed'));
      });
      renderWithProviders(<ResourceServerEditPage />);
      openAccessTab();
      await screen.findByTestId('organization-access-settings');
      await renameServer('Renamed server');

      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => expect(vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0].readOnly).toBe(false));
      expect(mockOrganizationAccess.save).not.toHaveBeenCalled();
    });

    it('unlocks the settings and keeps the draft when the access save fails after the server saved', async () => {
      mockOrganizationAccess.hasChanges = true;
      mockOrganizationAccess.save.mockRejectedValue(new Error('Request failed'));
      mockUpdateMutate.mockImplementation((_vars: unknown, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });
      const lastReadOnly = (): unknown => vi.mocked(OrganizationAccessSettings).mock.calls.at(-1)?.[0].readOnly;
      renderWithProviders(<ResourceServerEditPage />);
      openAccessTab();
      await screen.findByTestId('organization-access-settings');
      await renameServer('Renamed server');

      fireEvent.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => expect(mockOrganizationAccess.save).toHaveBeenCalled());
      await waitFor(() => expect(lastReadOnly()).toBe(false));
      expect(screen.getByRole('button', {name: 'Save'})).toBeEnabled();
      expect(screen.getByRole('button', {name: 'Reset'})).toBeEnabled();
      expect(mockOrganizationAccess.reset).not.toHaveBeenCalled();
    });

    it('shows the organization access save error and resets the draft', () => {
      mockOrganizationAccess.hasChanges = true;
      mockOrganizationAccess.saveError = new Error('Request failed');
      renderWithProviders(<ResourceServerEditPage />);

      expect(screen.getByText('Failed to update organization access. Please try again.')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: 'Reset'}));

      expect(mockOrganizationAccess.reset).toHaveBeenCalled();
    });
  });
});
