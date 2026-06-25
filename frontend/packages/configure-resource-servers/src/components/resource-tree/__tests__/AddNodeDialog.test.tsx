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

  it('renders the Add Tool title when mode is mcp-server-tool', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-server-tool" />);

    expect(screen.getByText('Add Tool')).toBeInTheDocument();
  });

  it('renders the Add Resource title when mode is mcp-server-resource', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-server-resource" />);

    expect(screen.getByText('Add Resource')).toBeInTheDocument();
  });

  it('renders the Add Namespace title when mode is mcp-namespace', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-namespace" />);

    expect(screen.getByText('Add Namespace')).toBeInTheDocument();
  });

  it('renders the Add Namespace title when mode is mcp-sub-namespace', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-sub-namespace" />);

    expect(screen.getByText('Add Namespace')).toBeInTheDocument();
  });

  it('renders the Add Tool title when mode is mcp-namespace-tool', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-namespace-tool" />);

    expect(screen.getByText('Add Tool')).toBeInTheDocument();
  });

  it('renders the Add Resource title when mode is mcp-namespace-resource', () => {
    renderWithProviders(<AddNodeDialog {...defaultProps} mode="mcp-namespace-resource" />);

    expect(screen.getByText('Add Resource')).toBeInTheDocument();
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

  it('does not send kind in the create payload for mcp-namespace mode', async () => {
    renderWithProviders(
      <AddNodeDialog {...defaultProps} mode="mcp-namespace" delimiter=":" parentResourceId={undefined} />,
    );

    const textboxes = screen.getAllByRole('textbox');
    fireEvent.change(textboxes[0], {target: {value: 'My Namespace'}});

    await waitFor(() => expect(textboxes[1]).toHaveValue('my-namespace'));

    fireEvent.click(screen.getByRole('button', {name: /^add$/i}));

    await waitFor(() => {
      expect(mockCreateResourceMutate).toHaveBeenCalledWith(
        expect.objectContaining({name: 'My Namespace', handle: 'my-namespace'}),
        expect.any(Object),
      );
      expect(mockCreateResourceMutate).toHaveBeenCalledWith(
        expect.not.objectContaining({kind: expect.anything() as unknown}),
        expect.any(Object),
      );
    });
  });

  it('sends kind=tool in the create payload for mcp-namespace-tool mode', async () => {
    renderWithProviders(
      <AddNodeDialog {...defaultProps} mode="mcp-namespace-tool" delimiter=":" parentResourceId="ns-1" />,
    );

    const textboxes = screen.getAllByRole('textbox');
    fireEvent.change(textboxes[0], {target: {value: 'Search'}});

    await waitFor(() => expect(textboxes[1]).toHaveValue('search'));

    fireEvent.click(screen.getByRole('button', {name: /^add$/i}));

    await waitFor(() => {
      expect(mockCreateActionMutate).toHaveBeenCalledWith(
        expect.objectContaining({kind: 'tool', name: 'Search', handle: 'search'}),
        expect.any(Object),
      );
    });
  });
});
