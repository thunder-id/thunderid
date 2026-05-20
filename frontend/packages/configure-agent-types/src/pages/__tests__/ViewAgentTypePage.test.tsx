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

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
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
    name: 'default',
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

    it('renders error message and back button on fetch error', () => {
      mockUseGetAgentType.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Boom'),
      });

      render(<ViewAgentTypePage />);

      expect(screen.getByText('Boom')).toBeInTheDocument();
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
  });

  describe('Save', () => {
    it('saves schema changes via the unsaved-changes bar', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));

      const saveButton = await screen.findByRole('button', {name: /^Save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            agentTypeId: 'schema-1',
            data: expect.objectContaining({
              name: 'default',
              ouId: 'ou-1',
            }) as Record<string, unknown>,
          }),
        );
      });
    });

    it('shows a toast on duplicate property names', async () => {
      const user = userEvent.setup();
      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Make Duplicate'));

      const saveButton = await screen.findByRole('button', {name: /^Save$/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'error');
      });

      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it('shows a toast on save error', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockRejectedValueOnce(new Error('Save failed'));

      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));
      await user.click(await screen.findByRole('button', {name: /^Save$/i}));

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith('Save failed', 'error');
      });
    });

    it('falls back to a generic error message for non-Error rejections', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockRejectedValueOnce('string error');

      render(<ViewAgentTypePage />);

      await user.click(screen.getByText('Update Properties'));
      await user.click(await screen.findByRole('button', {name: /^Save$/i}));

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith(expect.stringContaining('Failed to save agent type'), 'error');
      });
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
