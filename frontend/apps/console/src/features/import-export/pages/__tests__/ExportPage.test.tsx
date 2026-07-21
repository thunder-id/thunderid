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

import type {UseMutationResult} from '@tanstack/react-query';
import {render, screen, userEvent} from '@thunderid/test-utils';
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

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
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
      expect(screen.getByText('export.page.loading')).toBeInTheDocument();
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

    it('renders error alert', () => {
      render(<ExportPage />);
      expect(screen.getByRole('alert')).toBeInTheDocument();
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
      expect(screen.getByText('export.page.title')).toBeInTheDocument();
    });

    it('calls navigate(-1) when close button is clicked', async () => {
      const user = userEvent.setup();
      render(<ExportPage />);

      await user.click(screen.getByRole('button', {name: 'common:actions.close'}));

      expect(mockNavigate).toHaveBeenCalledWith(-1);
    });
  });
});
