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

const mockCreateResourceMutate = vi.fn();
const mockCreateActionMutate = vi.fn();

vi.mock('../../../api/useCreateResource', () => ({
  default: () => ({mutate: mockCreateResourceMutate, isPending: false}),
}));

vi.mock('../../../api/useCreateAction', () => ({
  default: () => ({mutate: mockCreateActionMutate, isPending: false}),
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

  it('shows an error and disables the Add button when the handle contains the delimiter character', async () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} delimiter="/" />);

    const textboxes = screen.getAllByRole('textbox');
    const handleInput = textboxes[1];

    fireEvent.change(handleInput, {target: {value: 'foo/bar'}});

    await waitFor(() => {
      expect(handleInput).toHaveValue('foo/bar');
    });

    expect(screen.getByText('Handle cannot contain the delimiter character "/".')).toBeInTheDocument();

    const addButton = screen.getByRole('button', {name: /^add$/i});
    expect(addButton).toBeDisabled();
  });

  it('renders the Add tool permission title when mode is mcp-server-tool', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-server-tool" />);

    expect(screen.getByText('Add tool permission')).toBeInTheDocument();
  });

  it('renders the Add resource permission title when mode is mcp-server-resource', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-server-resource" />);

    expect(screen.getByText('Add resource permission')).toBeInTheDocument();
  });

  it('sends kind=tool in the create payload for mcp-server-tool mode', async () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-server-tool" delimiter=":" />);

    const textboxes = screen.getAllByRole('textbox');
    fireEvent.change(textboxes[0], {target: {value: 'Search Files'}});

    await waitFor(() => expect(textboxes[1]).toHaveValue('search-files'));

    fireEvent.click(screen.getByRole('button', {name: /^add$/i}));

    await waitFor(() => {
      expect(mockCreateActionMutate).toHaveBeenCalledWith(
        expect.objectContaining({kind: 'tool', name: 'Search Files', handle: 'search-files'}),
        expect.any(Object),
      );
    });
  });

  it('sends kind=resource in the create payload for mcp-server-resource mode', async () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-server-resource" delimiter=":" />);

    const textboxes = screen.getAllByRole('textbox');
    fireEvent.change(textboxes[0], {target: {value: 'File Contents'}});

    await waitFor(() => expect(textboxes[1]).toHaveValue('file-contents'));

    fireEvent.click(screen.getByRole('button', {name: /^add$/i}));

    await waitFor(() => {
      expect(mockCreateActionMutate).toHaveBeenCalledWith(
        expect.objectContaining({kind: 'resource', name: 'File Contents', handle: 'file-contents'}),
        expect.any(Object),
      );
    });
  });

  it('shows generic resource placeholders in resource mode', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="resource" />);

    const textboxes = screen.getAllByRole('textbox');
    expect(textboxes[0]).toHaveAttribute('placeholder', 'e.g. Orders');
    expect(textboxes[1]).toHaveAttribute('placeholder', 'e.g. orders');
    expect(textboxes[2]).toHaveAttribute('placeholder', 'e.g. Manages order data and lifecycle');
  });

  it('shows action placeholders in server-action mode', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="server-action" />);

    const textboxes = screen.getAllByRole('textbox');
    expect(textboxes[0]).toHaveAttribute('placeholder', 'e.g. Read');
    expect(textboxes[1]).toHaveAttribute('placeholder', 'e.g. read');
    expect(textboxes[2]).toHaveAttribute('placeholder', 'e.g. Grants read access to the resource');
  });

  it('shows tool placeholders in mcp-server-tool mode', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-server-tool" />);

    const textboxes = screen.getAllByRole('textbox');
    expect(textboxes[0]).toHaveAttribute('placeholder', 'e.g. Send message');
    expect(textboxes[1]).toHaveAttribute('placeholder', 'e.g. send-message');
    expect(textboxes[2]).toHaveAttribute('placeholder', 'e.g. Sends a message to the specified channel');
  });
});
