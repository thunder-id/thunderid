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
import type {ResourceServer} from '../../../models/resource-server';
import AdvancedTab from '../AdvancedTab';

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

const mockUpdateMutate = vi.fn();

vi.mock('../../../api/useUpdateResourceServer', () => ({
  default: () => ({mutate: mockUpdateMutate, isPending: false}),
}));

const mockResourceServer: ResourceServer = {
  id: 'rs-1',
  name: 'Test API',
  handle: 'test-api',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: ':',
  type: 'API',
};

const readOnlyResourceServer: ResourceServer = {
  ...mockResourceServer,
  isReadOnly: true,
};

describe('AdvancedTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the Configurations section with the current identifier value', () => {
    renderWithProviders(<AdvancedTab resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByLabelText(/Identifier/i)).toHaveValue('https://api.example.com');
  });

  it('shows Save and Discard buttons when the identifier field is edited', async () => {
    renderWithProviders(<AdvancedTab resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    const identifierInput = screen.getByLabelText(/Identifier/i);
    fireEvent.change(identifierInput, {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Save/i})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Discard/i})).toBeInTheDocument();
    });
  });

  it('calls updateRs.mutate with the new identifier when Save is clicked', async () => {
    renderWithProviders(<AdvancedTab resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    const identifierInput = screen.getByLabelText(/Identifier/i);
    fireEvent.change(identifierInput, {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Save/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Save/i}));

    expect(mockUpdateMutate).toHaveBeenCalledWith(
      {id: 'rs-1', data: {identifier: 'https://new-api.example.com'}},
      expect.any(Object),
    );
  });

  it('resets the identifier to the original value when Discard is clicked', async () => {
    renderWithProviders(<AdvancedTab resourceServer={mockResourceServer} onRefresh={vi.fn()} />);

    const identifierInput = screen.getByLabelText(/Identifier/i);
    fireEvent.change(identifierInput, {target: {value: 'https://new-api.example.com'}});

    await waitFor(() => {
      expect(screen.getByRole('button', {name: /Discard/i})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: /Discard/i}));

    expect(identifierInput).toHaveValue('https://api.example.com');
  });

  it('disables the identifier field for read-only resource servers', () => {
    renderWithProviders(<AdvancedTab resourceServer={readOnlyResourceServer} onRefresh={vi.fn()} />);

    expect(screen.getByLabelText(/Identifier/i)).toBeDisabled();
  });
});
