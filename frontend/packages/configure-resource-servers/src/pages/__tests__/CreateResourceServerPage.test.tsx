// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ResourceServer} from '../../models/resource-server';
import CreateResourceServerPage from '../CreateResourceServerPage';

const mockNavigate = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    useThunderID: () => ({http: {request: vi.fn()}}),
  };
});

const mockShowToast = vi.fn();

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090', config: {brand: {product_name: 'ThunderID'}}}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('@thunderid/utils', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/utils')>();
  return {
    ...actual,
    generateRandomHumanReadableIdentifiers: () => ['Alpha Service', 'Beta Platform'],
  };
});

const mockCreateResourceServerMutate = vi.fn();
const mockCreateResourceServerReset = vi.fn();
const mockUseCreateResourceServer = vi.fn(() => ({
  mutate: mockCreateResourceServerMutate,
  isPending: false,
  isError: false,
  reset: mockCreateResourceServerReset,
}));

vi.mock('../../api/useCreateResourceServer', () => ({
  default: () => mockUseCreateResourceServer(),
}));

const mockSetDefaultMutate = vi.fn();
const mockDefaultConfig = vi.fn(() => ({
  data: {readOnly: {}, writable: {}, merged: {}},
  isLoading: false,
  error: null,
}));

vi.mock('../../api/useGetDefaultResourceServer', () => ({
  default: () => mockDefaultConfig(),
}));

const mockUseSetDefaultResourceServer = vi.fn(() => ({mutate: mockSetDefaultMutate, isPending: false}));

vi.mock('../../api/useSetDefaultResourceServer', () => ({
  default: () => mockUseSetDefaultResourceServer(),
}));

const mockUseHasMultipleOUs = vi.fn(() => ({
  hasMultipleOUs: false,
  isLoading: false,
  ouList: [{id: 'ou-1', name: 'Default', handle: 'default', parent: null}],
}));

vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: () => mockUseHasMultipleOUs(),
  OrganizationUnitTreePicker: () => <div data-testid="ou-tree-picker" />,
  OrganizationUnitPickerScreen: ({
    value,
    onChange,
    onBack,
    onContinue,
    backLabel,
    continueLabel,
  }: {
    value: string;
    onChange: (id: string) => void;
    onBack: () => void;
    onContinue: () => void;
    backLabel: string;
    continueLabel: string;
  }) => (
    <div data-testid="organization-unit-picker-screen">
      <button type="button" onClick={() => onChange('ou-2')}>
        Select OU
      </button>
      <span>{value}</span>
      <button type="button" onClick={onBack}>
        {backLabel}
      </button>
      <button type="button" onClick={onContinue}>
        {continueLabel}
      </button>
    </div>
  ),
}));

describe('CreateResourceServerPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseCreateResourceServer.mockReturnValue({
      mutate: mockCreateResourceServerMutate,
      isPending: false,
      isError: false,
      reset: mockCreateResourceServerReset,
    });
    mockDefaultConfig.mockReturnValue({
      data: {readOnly: {}, writable: {}, merged: {}},
      isLoading: false,
      error: null,
    });
    mockUseSetDefaultResourceServer.mockReturnValue({mutate: mockSetDefaultMutate, isPending: false});
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [{id: 'ou-1', name: 'Default', handle: 'default', parent: null}],
    });
  });

  it('renders the Type step initially', () => {
    renderWithProviders(<CreateResourceServerPage />);

    expect(screen.getByText(/What type of resource server are you adding/i)).toBeInTheDocument();
  });

  it('renders the type cards in the Type step', () => {
    renderWithProviders(<CreateResourceServerPage />);

    expect(screen.getAllByRole('button', {name: /API|MCP|Custom/i}).length).toBeGreaterThanOrEqual(1);
  });

  it('advances to the Name step after selecting a type', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    const apiCard = screen.getByRole('button', {name: /API/i});
    fireEvent.click(apiCard);

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });
  });

  it('shows the Name step with name and identifier fields after navigating to it', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    const apiCard = screen.getByRole('button', {name: /API/i});
    fireEvent.click(apiCard);
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
      expect(screen.getByRole('textbox', {name: /identifier/i})).toBeInTheDocument();
    });
  });

  it('keeps Continue disabled on the Name step until both name and identifier are filled', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });

    expect(screen.getByRole('button', {name: /Continue/i})).toBeDisabled();

    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });

    expect(screen.getByRole('button', {name: /Continue/i})).toBeEnabled();
  });

  it('advances to the Separator step after filling the name and identifier and clicking Next', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  it('shows the permission preview in the Separator step', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByText('<resource>:<action>')).toBeInTheDocument();
    });
  });

  it('shows the MCP server name label after selecting the MCP type', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /MCP/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /mcp server name/i})).toBeInTheDocument();
    });
  });

  it('shows the Create MCP server submit button on the last step after selecting the MCP type', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /MCP/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /mcp server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /mcp server name/i}), {
      target: {value: 'Payments MCP'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://mcp.example.com'},
    });

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Create MCP server/i})).toBeInTheDocument();
    });
  });

  it('shows the Create resource server submit button on the last step after selecting the API type', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Create resource server/i})).toBeInTheDocument();
    });
  });

  it('sends the identifier in the create payload', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Create resource server/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Create resource server/i}));

    expect(mockCreateResourceServerMutate).toHaveBeenCalledWith(
      expect.objectContaining({name: 'Payments API', identifier: 'https://api.example.com'}),
      expect.any(Object),
    );
    expect(mockCreateResourceServerMutate.mock.calls[0][0]).not.toHaveProperty('handle');
  });

  it('shows the MCP server created success toast after a successful MCP server creation', async () => {
    mockCreateResourceServerMutate.mockImplementationOnce(
      (_payload: unknown, options: {onSuccess: (created: ResourceServer) => void}) => {
        options.onSuccess({id: 'mcp-rs-1'} as ResourceServer);
      },
    );

    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /MCP/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /mcp server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /mcp server name/i}), {
      target: {value: 'Payments MCP'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://mcp.example.com'},
    });

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Create MCP server/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Create MCP server/i}));

    await waitFor(() => {
      expect(mockShowToast).toHaveBeenCalledWith('MCP server created successfully.', 'success');
      expect(mockNavigate).toHaveBeenCalledWith('/resource-servers/mcp-rs-1?tab=resources');
    });
  });

  it('displays the resolved catalog message, never the raw server text, when create fails', async () => {
    const rawServerMessage = 'raw backend create failure detail';
    mockCreateResourceServerMutate.mockImplementation((_payload: unknown, options: {onError: (err: Error) => void}) => {
      options.onError(new Error(rawServerMessage));
    });

    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Create resource server/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Create resource server/i}));

    await waitFor(() => {
      expect(screen.getByText('Failed to create resource server. Please try again.')).toBeInTheDocument();
    });
    expect(screen.queryByText(rawServerMessage)).not.toBeInTheDocument();
  });

  it('resets the create error when the type is changed after a failed create', () => {
    mockUseCreateResourceServer.mockReturnValue({
      mutate: mockCreateResourceServerMutate,
      isPending: false,
      isError: true,
      reset: mockCreateResourceServerReset,
    });

    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));

    expect(mockCreateResourceServerReset).toHaveBeenCalled();
  });

  it('does not reset a pending create mutation when the type is changed', () => {
    mockUseCreateResourceServer.mockReturnValue({
      mutate: mockCreateResourceServerMutate,
      isPending: true,
      isError: false,
      reset: mockCreateResourceServerReset,
    });

    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));

    expect(mockCreateResourceServerReset).not.toHaveBeenCalled();
  });

  describe('with multiple organization units', () => {
    beforeEach(() => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: true,
        isLoading: false,
        ouList: [
          {id: 'ou-1', name: 'Default', handle: 'default', parent: null},
          {id: 'ou-2', name: 'Other', handle: 'other', parent: null},
        ],
      });
    });

    it('shows the organization unit picker before the Type step', () => {
      renderWithProviders(<CreateResourceServerPage />);

      expect(screen.getByTestId('organization-unit-picker-screen')).toBeInTheDocument();
      expect(screen.queryByText(/What type of resource server are you adding/i)).not.toBeInTheDocument();
    });

    it('advances to the Type step after picking an organization unit', () => {
      renderWithProviders(<CreateResourceServerPage />);

      fireEvent.click(screen.getByRole('button', {name: 'Select OU'}));
      fireEvent.click(screen.getByRole('button', {name: 'Continue'}));

      expect(screen.getByText(/What type of resource server are you adding/i)).toBeInTheDocument();
    });

    it('sends the picked organization unit in the create payload', async () => {
      renderWithProviders(<CreateResourceServerPage />);

      fireEvent.click(screen.getByRole('button', {name: 'Select OU'}));
      fireEvent.click(screen.getByRole('button', {name: 'Continue'}));

      fireEvent.click(screen.getByRole('button', {name: /API/i}));
      fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

      await waitFor(() => {
        expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
      });

      fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
        target: {value: 'Payments API'},
      });
      fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
        target: {value: 'https://api.example.com'},
      });

      fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /Create resource server/i})).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', {name: /Create resource server/i}));

      expect(mockCreateResourceServerMutate).toHaveBeenCalledWith(
        expect.objectContaining({ouId: 'ou-2'}),
        expect.any(Object),
      );
    });
  });
});

describe('CreateResourceServerPage default resource server', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseCreateResourceServer.mockReturnValue({
      mutate: mockCreateResourceServerMutate,
      isPending: false,
      isError: false,
      reset: mockCreateResourceServerReset,
    });
    mockDefaultConfig.mockReturnValue({
      data: {readOnly: {}, writable: {}, merged: {}},
      isLoading: false,
      error: null,
    });
    mockUseSetDefaultResourceServer.mockReturnValue({mutate: mockSetDefaultMutate, isPending: false});
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [{id: 'ou-1', name: 'Default', handle: 'default', parent: null}],
    });
  });

  const defaultCheckbox = () => screen.queryByRole('checkbox', {name: /make this the default resource server/i});

  const goToLastStep = async (): Promise<void> => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });
  };

  const submit = async (): Promise<void> => {
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Create resource server/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Create resource server/i}));
  };

  const createSucceedsWith = (id: string): void => {
    mockCreateResourceServerMutate.mockImplementationOnce(
      (_payload: unknown, options: {onSuccess: (created: ResourceServer) => void}) => {
        options.onSuccess({id} as ResourceServer);
      },
    );
  };

  it('ticks the choice when the deployment has no default yet', async () => {
    await goToLastStep();

    expect(defaultCheckbox()).toBeChecked();
  });

  it('leaves the choice unticked when a default already exists', async () => {
    mockDefaultConfig.mockReturnValue({
      data: {readOnly: {}, writable: {resourceServerId: 'rs-existing'}, merged: {resourceServerId: 'rs-existing'}},
      isLoading: false,
      error: null,
    });

    await goToLastStep();

    expect(defaultCheckbox()).not.toBeChecked();
  });

  it('does not offer the choice when a declarative default has locked it', async () => {
    mockDefaultConfig.mockReturnValue({
      data: {readOnly: {resourceServerId: 'rs-locked'}, writable: {}, merged: {resourceServerId: 'rs-locked'}},
      isLoading: false,
      error: null,
    });

    await goToLastStep();

    expect(defaultCheckbox()).toBeNull();
  });

  it('does not offer the choice while the default config is still loading', async () => {
    mockDefaultConfig.mockReturnValue({data: undefined, isLoading: true, error: null});

    await goToLastStep();

    expect(defaultCheckbox()).toBeNull();
  });

  it('makes the created resource server the default when the choice is ticked', async () => {
    createSucceedsWith('rs-new');

    await goToLastStep();
    await submit();

    expect(mockSetDefaultMutate).toHaveBeenCalledWith({resourceServerId: 'rs-new'}, expect.any(Object));
  });

  it('leaves the default alone when the choice is unticked', async () => {
    createSucceedsWith('rs-new');

    await goToLastStep();
    fireEvent.click(defaultCheckbox()!);
    await submit();

    expect(mockCreateResourceServerMutate).toHaveBeenCalled();
    expect(mockSetDefaultMutate).not.toHaveBeenCalled();
  });

  it('still navigates to the created resource server when making it the default fails', async () => {
    createSucceedsWith('rs-new');
    mockSetDefaultMutate.mockImplementationOnce((_payload: unknown, options: {onError: (err: Error) => void}) => {
      options.onError(new Error('locked'));
    });

    await goToLastStep();
    await submit();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(expect.stringContaining('rs-new'));
    });
    expect(mockShowToast).toHaveBeenCalledWith(expect.stringMatching(/could not be made the default/i), 'warning');
  });
});

describe('CreateResourceServerPage submit locking', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseCreateResourceServer.mockReturnValue({
      mutate: mockCreateResourceServerMutate,
      isPending: false,
      isError: false,
      reset: mockCreateResourceServerReset,
    });
    mockDefaultConfig.mockReturnValue({
      data: {readOnly: {}, writable: {}, merged: {}},
      isLoading: false,
      error: null,
    });
    mockUseSetDefaultResourceServer.mockReturnValue({mutate: mockSetDefaultMutate, isPending: false});
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [{id: 'ou-1', name: 'Default', handle: 'default', parent: null}],
    });
  });

  const goToLastStep = async (): Promise<void> => {
    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });
    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Create resource server/i})).toBeInTheDocument();
    });
  };

  const submitButton = (): HTMLElement => screen.getByRole('button', {name: /Create resource server|Creating/i});

  it('keeps the submit button disabled while the follow-up default request is still in flight', async () => {
    const {rerender} = renderWithProviders(<CreateResourceServerPage />);
    await goToLastStep();

    expect(submitButton()).toBeEnabled();

    // The create has resolved but its follow-up default request has not, so navigation is still
    // pending. Re-submitting here would create a duplicate resource server.
    mockUseCreateResourceServer.mockReturnValue({
      mutate: mockCreateResourceServerMutate,
      isPending: false,
      isSuccess: true,
      isError: false,
      reset: mockCreateResourceServerReset,
    });
    mockUseSetDefaultResourceServer.mockReturnValue({mutate: mockSetDefaultMutate, isPending: true});
    rerender(<CreateResourceServerPage />);

    expect(submitButton()).toBeDisabled();
  });

  it('keeps the submit button disabled after a create succeeds even before the default request starts', async () => {
    const {rerender} = renderWithProviders(<CreateResourceServerPage />);
    await goToLastStep();

    mockUseCreateResourceServer.mockReturnValue({
      mutate: mockCreateResourceServerMutate,
      isPending: false,
      isSuccess: true,
      isError: false,
      reset: mockCreateResourceServerReset,
    });
    rerender(<CreateResourceServerPage />);

    expect(submitButton()).toBeDisabled();
  });

  describe('Back navigation', () => {
    it('returns from the Separator step to the Name step', async () => {
      renderWithProviders(<CreateResourceServerPage />);

      fireEvent.click(screen.getByRole('button', {name: /API/i}));
      fireEvent.click(screen.getByRole('button', {name: /Continue/i}));
      await waitFor(() => {
        expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
      });
      fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
        target: {value: 'Payments API'},
      });
      fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
        target: {value: 'https://api.example.com'},
      });
      fireEvent.click(screen.getByRole('button', {name: /Continue/i}));
      await waitFor(() => {
        expect(screen.getByRole('combobox')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', {name: /Back/i}));

      await waitFor(() => {
        expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
      });
    });

    it('returns from the Name step to the Type step', async () => {
      renderWithProviders(<CreateResourceServerPage />);

      fireEvent.click(screen.getByRole('button', {name: /API/i}));
      fireEvent.click(screen.getByRole('button', {name: /Continue/i}));
      await waitFor(() => {
        expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', {name: /Back/i}));

      expect(screen.getByText(/What type of resource server are you adding/i)).toBeInTheDocument();
    });

    it('returns from the Type step to the organization unit picker when there are multiple OUs', () => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: true,
        isLoading: false,
        ouList: [
          {id: 'ou-1', name: 'Default', handle: 'default', parent: null},
          {id: 'ou-2', name: 'Other', handle: 'other', parent: null},
        ],
      });

      renderWithProviders(<CreateResourceServerPage />);

      fireEvent.click(screen.getByRole('button', {name: 'Select OU'}));
      fireEvent.click(screen.getByRole('button', {name: 'Continue'}));
      expect(screen.getByText(/What type of resource server are you adding/i)).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: /Back/i}));

      expect(screen.getByTestId('organization-unit-picker-screen')).toBeInTheDocument();
    });

    it('jumps back to an earlier step via its breadcrumb', async () => {
      renderWithProviders(<CreateResourceServerPage />);

      fireEvent.click(screen.getByRole('button', {name: /API/i}));
      fireEvent.click(screen.getByRole('button', {name: /Continue/i}));
      await waitFor(() => {
        expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Type'));

      expect(screen.getByText(/What type of resource server are you adding/i)).toBeInTheDocument();
    });
  });
});
