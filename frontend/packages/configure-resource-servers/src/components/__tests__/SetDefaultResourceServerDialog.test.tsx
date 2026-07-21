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

import {renderWithProviders, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {ResourceServer} from '../../models/resource-server';
import SetDefaultResourceServerDialog from '../SetDefaultResourceServerDialog';

const mockShowToast = vi.fn();

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useToast: () => ({showToast: mockShowToast}),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

const mockMutate = vi.fn();

vi.mock('../../api/useSetDefaultResourceServer', () => ({
  default: () => ({mutate: mockMutate, isPending: false}),
}));

const resourceServer: ResourceServer = {
  id: 'rs-1',
  name: 'Payments API',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: ':',
  type: 'API',
};

describe('SetDefaultResourceServerDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the title and the target server name', () => {
    renderWithProviders(<SetDefaultResourceServerDialog open resourceServer={resourceServer} onClose={vi.fn()} />);

    expect(screen.getByText('Set default resource server')).toBeInTheDocument();
    expect(screen.getByText('Payments API')).toBeInTheDocument();
  });

  it('does not render when closed', () => {
    renderWithProviders(
      <SetDefaultResourceServerDialog open={false} resourceServer={resourceServer} onClose={vi.fn()} />,
    );

    expect(screen.queryByText('Set default resource server')).not.toBeInTheDocument();
  });

  it('mutates with the resource server id when confirmed', () => {
    renderWithProviders(<SetDefaultResourceServerDialog open resourceServer={resourceServer} onClose={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));

    expect(mockMutate).toHaveBeenCalledWith({resourceServerId: 'rs-1'}, expect.any(Object));
  });

  it('shows a success toast and closes on a successful mutation', () => {
    mockMutate.mockImplementation((_vars, opts: {onSuccess: () => void}) => opts.onSuccess());
    const onClose = vi.fn();
    const onSuccess = vi.fn();

    renderWithProviders(
      <SetDefaultResourceServerDialog open resourceServer={resourceServer} onClose={onClose} onSuccess={onSuccess} />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));

    expect(mockShowToast).toHaveBeenCalledWith('Payments API is now the default resource server.', 'success');
    expect(onSuccess).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('shows an error toast on a failed mutation', () => {
    mockMutate.mockImplementation((_vars, opts: {onError: (err: Error) => void}) => opts.onError(new Error('nope')));

    renderWithProviders(<SetDefaultResourceServerDialog open resourceServer={resourceServer} onClose={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));

    expect(mockShowToast).toHaveBeenCalledWith('Failed to set the default resource server.', 'error');
  });
});
