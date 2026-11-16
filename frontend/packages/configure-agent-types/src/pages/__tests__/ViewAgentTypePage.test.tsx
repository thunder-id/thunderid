// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider} from '@thunderid/contexts';
import {render, screen, waitFor, within, userEvent} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ApiAgentType} from '../../models/agent-type';
import ViewAgentTypePage from '../ViewAgentTypePage';

const {
  mockNavigate,
  mockUseGetAgentType,
  mockUseUpdateAgentType,
  mockMutateAsync,
  mockResetUpdate,
  mockShowToast,
  mockDeleteMutate,
  mockUseGetAllAgentTypes,
} = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  mockUseGetAgentType: vi.fn(),
  mockUseUpdateAgentType: vi.fn(),
  mockMutateAsync: vi.fn(),
  mockResetUpdate: vi.fn(),
  mockShowToast: vi.fn(),
  mockDeleteMutate: vi.fn(),
  mockUseGetAllAgentTypes: vi.fn(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({id: 'schema-1'}),
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

vi.mock('@/api/useGetAgentType', () => ({
  default: (id?: string): unknown => mockUseGetAgentType(id) as unknown,
}));

vi.mock('@/api/useUpdateAgentType', () => ({
  default: (): unknown => mockUseUpdateAgentType() as unknown,
}));

vi.mock('@/api/useGetAllAgentTypes', () => ({
  default: (): unknown => mockUseGetAllAgentTypes() as unknown,
}));

vi.mock('@/api/useDeleteAgentType', () => ({
  default: (): unknown => ({mutate: mockDeleteMutate, isPending: false, error: null, reset: vi.fn()}),
}));

vi.mock('@/components/edit-agent-type/schema-settings/EditSchemaSettings', () => ({
  default: ({
    onPropertiesChange,
    disabled = false,
  }: {
    onPropertiesChange: (props: unknown[]) => void;
    disabled?: boolean;
  }) => (
    <div data-testid="edit-schema-settings" data-disabled={String(disabled)}>
      {(['email', 'mobile'] as const).map((type) => (
        <button
          key={type}
          type="button"
          onClick={() =>
            onPropertiesChange([
              {
                id: '0',
                name: 'contact',
                displayName: '',
                type,
                required: true,
                unique: false,
                credential: false,
                enum: [],
                regex: '',
              },
            ])
          }
        >
          Clear {type} Regex
        </button>
      ))}
      <button
        type="button"
        onClick={() =>
          onPropertiesChange([
            {
              id: '0',
              name: 'email',
              displayName: '',
              type: 'string',
              required: true,
              unique: false,
              credential: false,
              enum: [],
              regex: '',
            },
            {
              id: '1',
              name: 'email',
              displayName: '',
              type: 'string',
              required: false,
              unique: false,
              credential: false,
              enum: [],
              regex: '',
            },
          ])
        }
      >
        Make Duplicate
      </button>
      <button
        type="button"
        onClick={() =>
          onPropertiesChange([
            {
              id: '0',
              name: 'newField',
              displayName: '',
              type: 'string',
              required: false,
              unique: false,
              credential: false,
              enum: [],
              regex: '',
            },
          ])
        }
      >
        Update Properties
      </button>
      <button
        type="button"
        onClick={() =>
          onPropertiesChange([
            {
              id: '0',
              name: 'email',
              displayName: '',
              type: 'string',
              required: true,
              unique: true,
              credential: false,
              enum: [],
              regex: '',
            },
            {
              id: '1',
              name: 'age',
              displayName: '',
              type: 'number',
              required: false,
              unique: false,
              credential: false,
              enum: [],
              regex: '',
            },
          ])
        }
      >
        Revert Properties
      </button>
      <button
        type="button"
        onClick={() =>
          onPropertiesChange([
            {
              id: '0',
              name: 'email',
              displayName: '',
              type: 'string',
              required: true,
              unique: true,
              credential: false,
              enum: [],
              regex: '',
            },
            {
              id: '1',
              name: 'age',
              displayName: '',
              type: 'number',
              required: false,
              unique: false,
              credential: false,
              enum: [],
              regex: '',
            },
            {
              id: '2',
              name: 'nickname',
              displayName: '',
              type: 'string',
              required: false,
              unique: false,
              credential: false,
              enum: [],
              regex: '',
            },
          ])
        }
      >
        Add Optional Field
      </button>
    </div>
  ),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useToast: () => ({showToast: mockShowToast}),
  };
});

// Allows everything except changing agent types, so a check against the wrong resource leaves the actions on.
const viewOnly = {
  can: (resource: string, action = 'view') => resource !== 'agenttype' || action === 'view',
  isLoading: false,
};

describe('ViewAgentTypePage', () => {
  const baseAgentType: ApiAgentType = {
    id: 'schema-1',
    handle: 'default',
    displayName: 'Default',
    ouId: 'ou-1',
    schema: {
      email: {type: 'string', required: true, unique: true},
      age: {type: 'number'},
    },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetAgentType.mockReturnValue({
      data: baseAgentType,
      isLoading: false,
      error: null,
    });
    mockUseUpdateAgentType.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      reset: mockResetUpdate,
    });
    mockUseGetAllAgentTypes.mockReturnValue({
      data: [{id: 'schema-1', handle: 'default', displayName: 'Default', ouId: 'ou-1', isReadOnly: false}],
      isLoading: false,
    });
  });

  describe('Loading and Error States', () => {
    it('renders a progressbar while loading', () => {
      mockUseGetAgentType.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
      });

      render(<ViewAgentTypePage />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('renders a resolved error message and back button on fetch error, not raw server text', () => {
      mockUseGetAgentType.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Boom'),
        refetch: vi.fn(),
      });

      render(<ViewAgentTypePage />);

      expect(screen.getByText('Failed to load agent type information')).toBeInTheDocument();
      expect(screen.queryByText('Boom')).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Back to agent types/i})).toBeInTheDocument();
    });

    it('renders a not-found message when no data is returned', () => {
      mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: false, error: null});

      render(<ViewAgentTypePage />);

      // The page falls back to a translation that resolves to "Agent type not found" via test-utils i18n
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Back to agent types/i})).toBeInTheDocument();
    });

    it('navigates back from error state', async () => {
      const user = userEvent.setup();
      mockUseGetAgentType.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Boom'),
      });

      render(<ViewAgentTypePage />);

      await user.click(screen.getByRole('button', {name: /Back to agent types/i}));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/agent-types');
      });
    });
  });

  describe('Header', () => {
    // A display name can be a translation template. One with no translation falls back to the handle.
    it('should resolve a translation template display name, and fall back to the handle', async () => {
      mockUseGetAgentType.mockReturnValue({
        data: {...baseAgentType, displayName: '{{t(custom:missingAgentType)}}'},
        isLoading: false,
        error: null,
      });
      render(<ViewAgentTypePage />);

      expect(screen.getByRole('heading', {level: 1, name: 'default'})).toBeInTheDocument();
      expect(screen.queryByText('{{t(custom:missingAgentType)}}')).not.toBeInTheDocument();
      await waitFor(() => expect(document.title).toBe('default | ThunderID'));
    });

    it('should render the agent type name as the heading', () => {
      render(<ViewAgentTypePage />);

      expect(screen.getByRole('heading', {level: 1, name: 'Default'})).toBeInTheDocument();
    });

    it('should set the browser tab title to the agent type name', async () => {
      render(<ViewAgentTypePage />);

      await waitFor(() => expect(document.title).toBe('Default | ThunderID'));
    });

    it('should link back to the agent types list', () => {
      render(<ViewAgentTypePage />);

      const backLink = screen.getByRole('link', {name: /Back to agent types/i});
      expect(backLink).toHaveAttribute('href', '/agent-types');
    });
  });

  describe('Tabs', () => {
    it('should open on the Schema tab', () => {
      render(<ViewAgentTypePage />);

      expect(screen.getByRole('tab', {name: 'Schema'})).toHaveAttribute('aria-selected', 'true');
      expect(screen.getByTestId('edit-schema-settings')).toBeInTheDocument();
      expect(screen.queryByText('Danger zone')).not.toBeInTheDocument();
    });

    it('should show the danger zone on the Advanced tab', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));

      expect(screen.getByText('Danger zone')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Delete'})).toBeInTheDocument();
      expect(screen.queryByTestId('edit-schema-settings')).not.toBeInTheDocument();
    });
  });

  describe('Delete', () => {
    const openDeleteDialog = async (user: ReturnType<typeof userEvent.setup>): Promise<HTMLElement> => {
      await user.click(screen.getByRole('tab', {name: 'Advanced'}));
      await user.click(screen.getByRole('button', {name: 'Delete'}));

      return screen.findByRole('dialog', {name: 'Delete agent type'});
    };

    it('should delete the agent type and return to the list', async () => {
      const user = userEvent.setup();
      mockNavigate.mockResolvedValue(undefined);
      mockDeleteMutate.mockImplementation((_id: string, options: {onSuccess: () => void}) => options.onSuccess());
      render(<ViewAgentTypePage />);

      const dialog = await openDeleteDialog(user);
      await user.click(within(dialog).getByRole('button', {name: 'Delete'}));

      expect(mockDeleteMutate).toHaveBeenCalledWith('schema-1', expect.any(Object));
      await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/agent-types'));
    });

    it('should keep the dialog open and explain why when the type is still in use', async () => {
      const user = userEvent.setup();
      mockDeleteMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) =>
        options.onError(Object.assign(new Error('Request failed'), {response: {data: {code: 'USRS-1022'}}})),
      );
      render(<ViewAgentTypePage />);

      const dialog = await openDeleteDialog(user);
      await user.click(within(dialog).getByRole('button', {name: 'Delete'}));

      expect(
        await within(dialog).findByText(
          'This agent type is in use. Delete or change the agents of this type, and remove it from the allowed agent types of applications, agents, and flows, before deleting it.',
        ),
      ).toBeInTheDocument();
      expect(screen.getByRole('dialog', {name: 'Delete agent type'})).toBeInTheDocument();
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('should not offer deleting when the admin may not delete agent types', async () => {
      const user = userEvent.setup();
      render(
        <CapabilitiesProvider capabilities={viewOnly}>
          <ViewAgentTypePage />
        </CapabilitiesProvider>,
      );

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));

      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    });

    // The detail response has no read-only flag, so it comes from the agent type list.
    it('should not offer deleting a read-only agent type', async () => {
      const user = userEvent.setup();
      mockUseGetAllAgentTypes.mockReturnValue({
        data: [{id: 'schema-1', handle: 'default', displayName: 'Default', ouId: 'ou-1', isReadOnly: true}],
        isLoading: false,
      });
      render(<ViewAgentTypePage />);

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));

      expect(screen.queryByText('Danger zone')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    });

    it('should show a read-only agent type without letting its schema be saved', async () => {
      const user = userEvent.setup();
      mockUseGetAllAgentTypes.mockReturnValue({
        data: [{id: 'schema-1', handle: 'default', displayName: 'Default', ouId: 'ou-1', isReadOnly: true}],
        isLoading: false,
      });
      render(<ViewAgentTypePage />);

      expect(screen.getByText('This resource is read-only and cannot be modified.')).toBeInTheDocument();
      expect(screen.getByTestId('edit-schema-settings')).toHaveAttribute('data-disabled', 'true');

      await user.click(screen.getByText('Update Properties'));

      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('should not offer deleting until the agent type list says whether the type is read-only', async () => {
      const user = userEvent.setup();
      mockUseGetAllAgentTypes.mockReturnValue({data: undefined, isLoading: true});
      render(<ViewAgentTypePage />);

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));

      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    });

    it('should keep the schema from being edited until the agent type list says the type is editable', () => {
      mockUseGetAllAgentTypes.mockReturnValue({data: undefined, isLoading: true});
      render(<ViewAgentTypePage />);

      expect(screen.getByTestId('edit-schema-settings')).toHaveAttribute('data-disabled', 'true');
      expect(screen.queryByText('This resource is read-only and cannot be modified.')).not.toBeInTheDocument();
    });

    it('should keep editing and deleting off, and offer a retry, when the agent type list fails', async () => {
      const user = userEvent.setup();
      const refetchAgentTypes = vi.fn().mockResolvedValue({});
      mockUseGetAllAgentTypes.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('list failed'),
        refetch: refetchAgentTypes,
      });
      render(<ViewAgentTypePage />);

      expect(
        screen.getByText('Could not check whether this agent type can be edited, so editing and deleting are off.'),
      ).toBeInTheDocument();
      expect(screen.getByTestId('edit-schema-settings')).toHaveAttribute('data-disabled', 'true');

      await user.click(screen.getByRole('button', {name: 'Refresh'}));
      expect(refetchAgentTypes).toHaveBeenCalledTimes(1);

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));
      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    });

    it('should close the dialog on cancel without deleting', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      const dialog = await openDeleteDialog(user);
      await user.click(within(dialog).getByRole('button', {name: 'Cancel'}));

      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(mockDeleteMutate).not.toHaveBeenCalled();
    });
  });

  describe('Schema settings child', () => {
    it('passes properties down through EditSchemaSettings', () => {
      render(<ViewAgentTypePage />);

      // The mocked EditSchemaSettings exposes a "Make Duplicate" button when it receives properties
      expect(screen.getByText('Make Duplicate')).toBeInTheDocument();
    });
  });

  describe('Schema Settings', () => {
    it('renders the EditSchemaSettings child', () => {
      render(<ViewAgentTypePage />);

      expect(screen.getByTestId('edit-schema-settings')).toBeInTheDocument();
    });
  });

  describe('Unsaved changes', () => {
    it('shows the unsaved-changes bar when properties change', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });
    });

    it('resets when Reset is clicked', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', {name: /Reset/i}));

      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });

    it('hides the unsaved-changes bar when properties are manually reverted to the server schema', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));
      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      // Edit back to the exact server schema — no real change remains.
      await user.click(screen.getByText('Revert Properties'));
      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });
  });

  describe('Save', () => {
    it('disables Save when the admin may not update agent types', async () => {
      const user = userEvent.setup();
      render(
        <CapabilitiesProvider capabilities={viewOnly}>
          <ViewAgentTypePage />
        </CapabilitiesProvider>,
      );

      await user.click(screen.getByText('Update Properties'));

      expect(await screen.findByRole('button', {name: /^Save$/i})).toBeDisabled();
    });

    it('saves schema changes via the unsaved-changes bar', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      const saveButton = await screen.findByRole('button', {name: /^Save$/i});
      await user.click(saveButton);

      // Editing the schema shows a warning; continue.
      await user.click(await screen.findByRole('button', {name: /^Continue$/i}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            agentTypeId: 'schema-1',
            data: expect.objectContaining({
              displayName: 'Default',
              ouId: 'ou-1',
            }) as Record<string, unknown>,
          }),
        );
      });
      const [{data: sentData}] = mockMutateAsync.mock.calls[0] as [{data: Record<string, unknown>}];
      expect(sentData).not.toHaveProperty('handle');
    });

    it('should keep the display attribute the server returned when saving', async () => {
      const user = userEvent.setup();
      mockUseGetAgentType.mockReturnValue({
        data: {...baseAgentType, systemAttributes: {display: 'email'}},
        isLoading: false,
        error: null,
      });
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Add Optional Field'));
      await user.click(await screen.findByRole('button', {name: /^Save$/i}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            data: expect.objectContaining({systemAttributes: {display: 'email'}}) as Record<string, unknown>,
          }),
        );
      });
    });

    // Update Properties replaces every property with `newField`, as a rename of the display property
    // would. Sending the old display attribute would make the API reject the save with USRS-1011.
    it('should drop a display attribute whose property was renamed or removed', async () => {
      const user = userEvent.setup();
      mockUseGetAgentType.mockReturnValue({
        data: {...baseAgentType, systemAttributes: {display: 'email'}},
        isLoading: false,
        error: null,
      });
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));
      await user.click(await screen.findByRole('button', {name: /^Save$/i}));
      await user.click(await screen.findByRole('button', {name: /^Continue$/i}));

      await waitFor(() => expect(mockMutateAsync).toHaveBeenCalled());
      const [{data: sentData}] = mockMutateAsync.mock.calls[0] as [{data: Record<string, unknown>}];
      expect(sentData).not.toHaveProperty('systemAttributes');
      expect(sentData['schema']).toHaveProperty('newField');
    });

    it('warns before saving a breaking schema change and can be cancelled', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      // Update Properties drops email and age, removing attributes is a breaking change.
      await user.click(screen.getByText('Update Properties'));
      await user.click(await screen.findByRole('button', {name: /^Save$/i}));

      // The confirmation dialog appears instead of saving immediately and lists the affected attributes.
      const dialog = await screen.findByRole('dialog');
      expect(within(dialog).getByText(/Confirm schema changes/i)).toBeInTheDocument();
      expect(within(dialog).getByText('email')).toBeInTheDocument();
      expect(within(dialog).getByText('age')).toBeInTheDocument();

      await user.click(within(dialog).getByRole('button', {name: /Cancel/i}));

      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it('does not warn when a schema change is not breaking', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      // Adding a new optional attribute keeps existing ones intact, not a breaking change.
      await user.click(screen.getByText('Add Optional Field'));
      await user.click(await screen.findByRole('button', {name: /^Save$/i}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalled();
      });
      expect(screen.queryByText(/Confirm schema changes/i)).not.toBeInTheDocument();
    });

    it.each(['email', 'mobile'] as const)(
      'validates a blank %s regex before showing a breaking-change warning',
      async (type) => {
        const user = userEvent.setup();
        render(<ViewAgentTypePage />);

        await user.click(screen.getByText(`Clear ${type} Regex`));
        await user.click(await screen.findByRole('button', {name: /^Save$/i}));

        expect(screen.getByText(`Enter a regular expression for ${type} attributes.`)).toBeInTheDocument();
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(mockMutateAsync).not.toHaveBeenCalled();
      },
    );

    it('shows the duplicate-property validation error inline, not as a toast', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Make Duplicate'));

      const saveButton = await screen.findByRole('button', {name: /^Save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(screen.getByText('Duplicate property names found: email')).toBeInTheDocument();
      });

      expect(mockMutateAsync).not.toHaveBeenCalled();
      expect(mockShowToast).not.toHaveBeenCalled();
    });

    it('shows the mutation error inline via the unsaved changes bar, not as a toast', async () => {
      const user = userEvent.setup();
      mockUseUpdateAgentType.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        error: new Error('Save failed'),
        isError: true,
        reset: mockResetUpdate,
      });

      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      await waitFor(() => {
        expect(screen.getByText('Failed to update agent type. Please try again.')).toBeInTheDocument();
      });
      expect(screen.queryByText('Save failed')).not.toBeInTheDocument();
      expect(mockShowToast).not.toHaveBeenCalled();
    });

    it('falls back to a generic error message for non-Error rejections', async () => {
      const user = userEvent.setup();
      mockUseUpdateAgentType.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        error: 'string error' as unknown as Error,
        isError: true,
        reset: mockResetUpdate,
      });

      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      await waitFor(() => {
        expect(screen.getByText('Failed to update agent type. Please try again.')).toBeInTheDocument();
      });
    });

    it('resets the save error as soon as the schema changes', async () => {
      const user = userEvent.setup();
      mockUseUpdateAgentType.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        error: new Error('Save failed'),
        isError: true,
        reset: mockResetUpdate,
      });

      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      expect(mockResetUpdate).toHaveBeenCalled();
    });

    it('shows the saving state while the mutation is pending', async () => {
      const user = userEvent.setup();
      mockUseUpdateAgentType.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: true,
        reset: mockResetUpdate,
      });

      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      await waitFor(() => {
        expect(screen.getByText(/Saving/i)).toBeInTheDocument();
      });
    });
  });
});
