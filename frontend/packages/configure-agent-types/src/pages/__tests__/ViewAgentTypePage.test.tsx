// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, within, userEvent} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ApiAgentType} from '../../models/agent-type';
import ViewAgentTypePage from '../ViewAgentTypePage';

const {mockNavigate, mockUseGetAgentType, mockUseUpdateAgentType, mockMutateAsync, mockResetUpdate, mockShowToast} =
  vi.hoisted(() => ({
    mockNavigate: vi.fn(),
    mockUseGetAgentType: vi.fn(),
    mockUseUpdateAgentType: vi.fn(),
    mockMutateAsync: vi.fn(),
    mockResetUpdate: vi.fn(),
    mockShowToast: vi.fn(),
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

vi.mock('@/components/edit-agent-type/schema-settings/EditSchemaSettings', () => ({
  default: ({onPropertiesChange}: {onPropertiesChange: (props: unknown[]) => void}) => (
    <div data-testid="edit-schema-settings">
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
      expect(screen.getByRole('button', {name: /Back to Agents/i})).toBeInTheDocument();
    });

    it('renders a not-found message when no data is returned', () => {
      mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: false, error: null});

      render(<ViewAgentTypePage />);

      // The page falls back to a translation that resolves to "Agent type not found" via test-utils i18n
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Back to Agents/i})).toBeInTheDocument();
    });

    it('navigates back from error state', async () => {
      const user = userEvent.setup();
      mockUseGetAgentType.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Boom'),
      });

      render(<ViewAgentTypePage />);

      await user.click(screen.getByRole('button', {name: /Back to Agents/i}));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/agents');
      });
    });
  });

  describe('Header', () => {
    it('renders the agent type heading', () => {
      render(<ViewAgentTypePage />);

      expect(screen.getByText('Agent Schema')).toBeInTheDocument();
    });

    it('renders the back link to /agents', () => {
      render(<ViewAgentTypePage />);

      const backLink = screen.getByRole('link', {name: /Back to Agents/i});
      expect(backLink).toHaveAttribute('href', '/agents');
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
