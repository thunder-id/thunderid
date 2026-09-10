// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {UseMutationResult} from '@tanstack/react-query';
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {ExportRequest, JSONExportResponse} from '../../models/export-configuration';

const mockNavigate = vi.fn();
const mockMutate = vi.fn();
const mockLogger = {error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn()};

let mockMutationState: Partial<UseMutationResult<JSONExportResponse, Error, ExportRequest>> = {
  mutate: mockMutate,
  data: undefined,
  isPending: false,
  isError: false,
  error: null,
};

vi.mock('../../api/useExportConfiguration', () => ({
  default: () => mockMutationState,
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => mockLogger,
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    X: () => <span data-testid="icon-x" />,
  };
});

vi.mock('../../components/ConfigureExport', () => ({
  default: ({resources, environmentVariables}: {resources: string; environmentVariables: string}) => (
    <div data-testid="configure-export" data-resources={resources} data-env={environmentVariables}>
      ConfigureExport
    </div>
  ),
}));

import ExportPage from '../ExportPage';

afterEach(() => {
  vi.clearAllMocks();
});

describe('ExportPage', () => {
  describe('on mount', () => {
    it('calls mutate with all resource wildcards', () => {
      render(<ExportPage />);

      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          applications: ['*'],
          connections: ['*'],
          flows: ['*'],
          groups: ['*'],
          agents: ['*'],
          serverConfigs: ['*'],
          credentialConfigurations: ['*'],
          presentationDefinitions: ['*'],
        }),
      );
    });
  });

  describe('loading state', () => {
    beforeEach(() => {
      mockMutationState = {...mockMutationState, isPending: true, data: undefined, isError: false} as Partial<
        UseMutationResult<JSONExportResponse, Error, ExportRequest>
      >;
    });

    it('renders loading indicator', () => {
      render(<ExportPage />);
      expect(screen.getByText('Loading export configuration...')).toBeInTheDocument();
    });

    it('does not render ConfigureExport while loading', () => {
      render(<ExportPage />);
      expect(screen.queryByTestId('configure-export')).not.toBeInTheDocument();
    });
  });

  describe('error state', () => {
    beforeEach(() => {
      mockMutationState = {
        ...mockMutationState,
        isPending: false,
        isError: true,
        error: new Error('Network error'),
        data: undefined,
      } as Partial<UseMutationResult<JSONExportResponse, Error, ExportRequest>>;
    });

    it('renders the resolved error message', () => {
      render(<ExportPage />);
      expect(screen.getByText('Failed to load export configuration')).toBeInTheDocument();
    });

    it('does not render ConfigureExport on error', () => {
      render(<ExportPage />);
      expect(screen.queryByTestId('configure-export')).not.toBeInTheDocument();
    });
  });

  describe('success state', () => {
    beforeEach(() => {
      mockMutationState = {
        mutate: mockMutate,
        isPending: false,
        isError: false,
        error: null,
        data: {
          resources: 'resource-content',
          environment_variables: 'ENV_VAR=value',
          summary: {totalFiles: 1, exported: {}, skipped: {}},
        } as unknown as JSONExportResponse,
      };
    });

    it('renders ConfigureExport with resource data', () => {
      render(<ExportPage />);
      const configureExport = screen.getByTestId('configure-export');
      expect(configureExport).toBeInTheDocument();
      expect(configureExport).toHaveAttribute('data-resources', 'resource-content');
    });

    it('passes environment_variables to ConfigureExport', () => {
      render(<ExportPage />);
      expect(screen.getByTestId('configure-export')).toHaveAttribute('data-env', 'ENV_VAR=value');
    });
  });

  describe('navigation', () => {
    it('renders page title', () => {
      render(<ExportPage />);
      expect(screen.getByText('Export Configuration')).toBeInTheDocument();
    });

    it('calls navigate(-1) when close button is clicked', async () => {
      const user = userEvent.setup();
      render(<ExportPage />);

      await user.click(screen.getByRole('button', {name: 'Close'}));

      expect(mockNavigate).toHaveBeenCalledWith(-1);
    });

    it('logs an error when navigating back fails', async () => {
      const user = userEvent.setup();
      mockNavigate.mockRejectedValueOnce(new Error('navigation failed'));
      render(<ExportPage />);

      await user.click(screen.getByRole('button', {name: 'Close'}));

      await waitFor(() => {
        expect(mockLogger.error).toHaveBeenCalledWith('Failed to navigate back', {error: expect.any(Error) as Error});
      });
    });

    it('navigates to the import/export list when the breadcrumb is clicked', async () => {
      const user = userEvent.setup();
      render(<ExportPage />);

      await user.click(screen.getByText('Import / Export'));

      expect(mockNavigate).toHaveBeenCalledWith('/import-export');
    });
  });
});
