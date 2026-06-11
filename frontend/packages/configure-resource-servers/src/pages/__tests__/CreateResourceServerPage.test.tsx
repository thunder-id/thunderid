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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CreateResourceServerPage from '../CreateResourceServerPage';

const mockNavigate = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
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

vi.mock('@thunderid/utils', () => ({
  generateRandomHumanReadableIdentifiers: () => ['Alpha Service', 'Beta Platform'],
}));

vi.mock('../../api/useCreateResourceServer', () => ({
  default: () => ({mutate: vi.fn(), isPending: false}),
}));

vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: () => ({
    hasMultipleOUs: false,
    isLoading: false,
    ouList: [{id: 'ou-1', name: 'Default', handle: 'default', parent: null}],
  }),
  OrganizationUnitTreePicker: () => <div data-testid="ou-tree-picker" />,
}));

describe('CreateResourceServerPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  it('shows the Name step with name and handle fields after navigating to it', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    const apiCard = screen.getByRole('button', {name: /API/i});
    fireEvent.click(apiCard);
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
      expect(screen.getByRole('textbox', {name: /handle/i})).toBeInTheDocument();
    });
  });

  it('advances to the Separator step after filling the name and clicking Next', async () => {
    renderWithProviders(<CreateResourceServerPage />);

    fireEvent.click(screen.getByRole('button', {name: /API/i}));
    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
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

    fireEvent.click(screen.getByRole('button', {name: /Continue/i}));

    await waitFor(() => {
      expect(screen.getByText(/payments-api/i)).toBeInTheDocument();
    });
  });
});
