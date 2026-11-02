// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen, fireEvent, waitFor, userEvent} from '@thunderid/test-utils';
import {FlowExecutionFailure} from '@thunderid/utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource, Action, ResourceListResponse, ActionListResponse} from '../../../models/resource-server';
import {ResourceNode, ActionNode} from '../ResourceTreeNode';

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: vi.fn()}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

const mockShowToast = vi.fn();

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

const mockDeleteResource = vi.fn();
const mockDeleteAction = vi.fn();

vi.mock('../../../api/useDeleteResource', () => ({
  default: () => ({mutate: mockDeleteResource, isPending: false}),
}));

vi.mock('../../../api/useDeleteAction', () => ({
  default: () => ({mutate: mockDeleteAction, isPending: false}),
}));

vi.mock('../../../api/useGetResources', () => ({
  default: () => ({data: {resources: []} as ResourceListResponse, isLoading: false}),
}));

vi.mock('../../../api/useGetResourceActions', () => ({
  default: () => ({data: {actions: []} as ActionListResponse, isLoading: false}),
}));

const mockResource: Resource = {
  id: 'r-1',
  name: 'Documents',
  handle: 'documents',
  permission: 'api/documents',
};

const mockAction: Action = {
  id: 'a-1',
  name: 'Read All',
  handle: 'read-all',
  permission: 'api:read-all',
};

describe('ResourceNode', () => {
  const defaultProps = {
    resourceServerId: 'rs-1',
    delimiter: '/',
    node: mockResource,
    depth: 1,
    selectedNodeId: null,
    onSelect: vi.fn(),
    onAddChild: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the resource name', () => {
    renderWithProviders(<ResourceNode {...defaultProps} />);

    expect(screen.getByText('Documents')).toBeInTheDocument();
  });

  it('does not render the permission chip', () => {
    renderWithProviders(<ResourceNode {...defaultProps} />);

    expect(screen.queryByText('api/documents')).not.toBeInTheDocument();
  });

  it('shows inline action controls on hover', async () => {
    renderWithProviders(<ResourceNode {...defaultProps} />);

    const nameEl = screen.getByText('Documents');
    fireEvent.mouseEnter(nameEl);

    await waitFor(() => {
      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThanOrEqual(2);
    });
  });

  it('shows a single Add button instead of separate sub-resource and action buttons on hover', async () => {
    renderWithProviders(<ResourceNode {...defaultProps} />);

    const nameEl = screen.getByText('Documents');
    fireEvent.mouseEnter(nameEl);

    await waitFor(() => {
      expect(screen.queryByRole('button', {name: /Add sub-resource/i})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: /Add action/i})).not.toBeInTheDocument();
    });
  });

  it('opens add menu with sub-resource and action options on Add button click', async () => {
    renderWithProviders(<ResourceNode {...defaultProps} />);

    const nameEl = screen.getByText('Documents');
    fireEvent.mouseEnter(nameEl);

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Add'})).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: 'Add'}));

    await waitFor(() => {
      expect(screen.getByText('Add sub-resource')).toBeInTheDocument();
      expect(screen.getByText('Add action')).toBeInTheDocument();
    });
  });

  it('calls onSelect with resource type when the row is clicked', () => {
    const onSelect = vi.fn();
    renderWithProviders(<ResourceNode {...defaultProps} onSelect={onSelect} />);

    fireEvent.click(screen.getByText('Documents'));

    expect(onSelect).toHaveBeenCalledWith({
      type: 'resource',
      id: 'r-1',
      data: mockResource,
    });
  });

  it('does not render a folder-open icon (icon changed to Layers)', () => {
    renderWithProviders(<ResourceNode {...defaultProps} />);

    const svgs = document.querySelectorAll('svg');
    const hasFolderOpenPath = Array.from(svgs).some((svg) => {
      const title = svg.querySelector('title');
      return title?.textContent === 'folder-open';
    });
    expect(hasFolderOpenPath).toBe(false);
  });

  it('renders a long resource name in full without truncation', () => {
    const longNameResource = {
      ...mockResource,
      name: 'This Is A Very Long Resource Name That Should Always Be Fully Visible',
    };
    renderWithProviders(<ResourceNode {...defaultProps} node={longNameResource} />);

    expect(
      screen.getByText('This Is A Very Long Resource Name That Should Always Be Fully Visible'),
    ).toBeInTheDocument();
  });

  it('shows the copy button tooltip with permission string text on hover', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ResourceNode {...defaultProps} />);

    const nameEl = screen.getByText('Documents');
    await user.hover(nameEl);

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Copy permission string'})).toBeInTheDocument();
    });
  });

  it('does not show the add or delete controls on hover when read-only', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ResourceNode {...defaultProps} readOnly />);

    const nameEl = screen.getByText('Documents');
    await user.hover(nameEl);

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Copy permission string'})).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', {name: 'Add'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
  });
});

describe('ActionNode', () => {
  const defaultProps = {
    resourceServerId: 'rs-1',
    action: mockAction,
    depth: 1,
    selectedNodeId: null,
    onSelect: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the action name', () => {
    renderWithProviders(<ActionNode {...defaultProps} />);

    expect(screen.getByText('Read All')).toBeInTheDocument();
  });

  it('does not render the action permission chip', () => {
    renderWithProviders(<ActionNode {...defaultProps} />);

    expect(screen.queryByText('api:read-all')).not.toBeInTheDocument();
  });

  it('calls onSelect with server-action type when no parentResourceId', () => {
    const onSelect = vi.fn();
    renderWithProviders(<ActionNode {...defaultProps} onSelect={onSelect} />);

    fireEvent.click(screen.getByText('Read All'));

    expect(onSelect).toHaveBeenCalledWith({
      type: 'server-action',
      id: 'a-1',
      data: mockAction,
      parentResourceId: undefined,
      breadcrumb: [],
    });
  });

  it('calls onSelect with resource-action type when parentResourceId is provided', () => {
    const onSelect = vi.fn();
    renderWithProviders(<ActionNode {...defaultProps} onSelect={onSelect} parentResourceId="r-1" />);

    fireEvent.click(screen.getByText('Read All'));

    expect(onSelect).toHaveBeenCalledWith({
      type: 'resource-action',
      id: 'a-1',
      data: mockAction,
      parentResourceId: 'r-1',
      breadcrumb: [],
    });
  });

  it('renders a long action name in full without truncation', () => {
    const longNameAction = {
      ...mockAction,
      name: 'This Is A Very Long Action Name That Should Always Be Fully Visible',
    };
    renderWithProviders(<ActionNode {...defaultProps} action={longNameAction} />);

    expect(screen.getByText('This Is A Very Long Action Name That Should Always Be Fully Visible')).toBeInTheDocument();
  });

  it('shows the copy button tooltip with permission string text on hover', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ActionNode {...defaultProps} />);

    const nameEl = screen.getByText('Read All');
    await user.hover(nameEl);

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Copy permission string'})).toBeInTheDocument();
    });
  });

  it('shows the reason a deletion flow refused the action', async () => {
    const user = userEvent.setup();
    mockDeleteAction.mockImplementation((_actionId: string, options: {onError: (error: Error) => void}) =>
      options.onError(new FlowExecutionFailure({code: 'FET-1108'}, 'The action deletion flow did not complete')),
    );
    renderWithProviders(<ActionNode {...defaultProps} selectedNodeId={mockAction.id} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    expect(mockShowToast).toHaveBeenCalledWith('This action cannot be deleted in its current state.', 'error');
  });

  it('does not show the delete control on hover when read-only', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ActionNode {...defaultProps} readOnly />);

    const nameEl = screen.getByText('Read All');
    await user.hover(nameEl);

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Copy permission string'})).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
  });
});
