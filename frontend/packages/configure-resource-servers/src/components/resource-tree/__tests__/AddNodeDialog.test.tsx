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
import AddNodeDialog from '../AddNodeDialog';

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

vi.mock('../../../api/useCreateResource', () => ({
  default: () => ({mutate: vi.fn(), isPending: false}),
}));

vi.mock('../../../api/useCreateAction', () => ({
  default: () => ({mutate: vi.fn(), isPending: false}),
}));

const defaultProps = {
  open: true,
  mode: 'resource' as const,
  resourceServerId: 'rs-1',
  parentPermission: 'dark-dodos',
  delimiter: '/',
  onClose: vi.fn(),
  onSuccess: vi.fn(),
};

describe('AddNodeDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the Add Resource title when mode is resource', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    expect(screen.getByText('Add Resource')).toBeInTheDocument();
  });

  it('renders the Name label when the dialog is open', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    expect(screen.getByText('Name')).toBeInTheDocument();
  });

  it('renders the Handle label when the dialog is open', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    expect(screen.getByText('Handle')).toBeInTheDocument();
  });

  it('renders the Description label when the dialog is open', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    expect(screen.getByText('Description')).toBeInTheDocument();
  });

  it('auto-derives the handle from the name when the name field is typed into', async () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    const textboxes = screen.getAllByRole('textbox');
    const nameInput = textboxes[0];
    const handleInput = textboxes[1];

    fireEvent.change(nameInput, {target: {value: 'My Resource'}});

    await waitFor(() => {
      expect(handleInput).toHaveValue('my-resource');
    });
  });

  it('stops auto-deriving the handle after the handle field is manually edited', async () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    const textboxes = screen.getAllByRole('textbox');
    const nameInput = textboxes[0];
    const handleInput = textboxes[1];

    fireEvent.change(handleInput, {target: {value: 'custom-handle'}});
    fireEvent.change(nameInput, {target: {value: 'Something New'}});

    await waitFor(() => {
      expect(handleInput).toHaveValue('custom-handle');
    });
  });

  it('shows the permission preview chip when the handle is non-empty', async () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    const textboxes = screen.getAllByRole('textbox');
    const nameInput = textboxes[0];

    fireEvent.change(nameInput, {target: {value: 'Docs'}});

    await waitFor(() => {
      expect(screen.getByText('dark-dodos/docs')).toBeInTheDocument();
    });
  });

  it('does not show the permission preview when the handle is empty', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} />);

    expect(screen.queryByText(/dark-dodos\//)).not.toBeInTheDocument();
  });

  it('renders the Add Action title when mode is server-action', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="server-action" />);

    expect(screen.getByText('Add Action')).toBeInTheDocument();
  });

  it('does not render when open is false', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} open={false} />);

    expect(screen.queryByText('Add Resource')).not.toBeInTheDocument();
  });
});
