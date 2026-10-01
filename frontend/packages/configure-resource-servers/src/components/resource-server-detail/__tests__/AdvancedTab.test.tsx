// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {AuthorizationEngines, type ResourceServer} from '../../../models/resource-server';
import AdvancedTab from '../AdvancedTab';

let mockPDPConnections: {
  data: {id: string; name: string}[];
  isLoading: boolean;
  error: Error | null;
} = {data: [{id: 'pdp-1', name: 'AuthZEN PDP'}], isLoading: false, error: null};
vi.mock('../../../api/useAuthZENPDPConnections', () => ({default: () => mockPDPConnections}));

const engineProps = {
  authorizationEngine: AuthorizationEngines.RBAC,
  pdpConnectionId: '',
  onAuthorizationEngineChange: vi.fn(),
  onPDPConnectionChange: vi.fn(),
};

const mockResourceServer: ResourceServer = {
  id: 'rs-1',
  name: 'Test API',
  description: 'Existing API description',
  identifier: 'https://api.example.com',
  ouId: 'ou-1',
  delimiter: ':',
  type: 'API',
};

const readOnlyResourceServer: ResourceServer = {
  ...mockResourceServer,
  isReadOnly: true,
};

const mockMcpServer: ResourceServer = {
  ...mockResourceServer,
  id: 'rs-2',
  name: 'Test MCP Server',
  type: 'MCP',
};

describe('AdvancedTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPDPConnections = {data: [{id: 'pdp-1', name: 'AuthZEN PDP'}], isLoading: false, error: null};
  });

  it('renders the Configurations section with the current identifier value', () => {
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.getByLabelText(/Identifier/i)).toHaveValue('https://api.example.com');
  });

  it('renders the identifier label as a top FormLabel, not a floating label', () => {
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    const label = screen.getByText('Identifier (Audience)');
    expect(label.tagName.toLowerCase()).toBe('label');
    expect(label).toHaveClass('MuiFormLabel-root');
  });

  it('calls onIdentifierChange when the identifier field is edited', () => {
    const onIdentifierChange = vi.fn();
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        onIdentifierChange={onIdentifierChange}
      />,
    );

    const identifierInput = screen.getByLabelText(/Identifier/i);
    fireEvent.change(identifierInput, {target: {value: 'https://new-api.example.com'}});

    expect(onIdentifierChange).toHaveBeenCalledWith('https://new-api.example.com');
  });

  it('reflects the identifier prop value in the field', () => {
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier="https://controlled.example.com"
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.getByLabelText(/Identifier/i)).toHaveValue('https://controlled.example.com');
  });

  it('disables the identifier field for read-only resource servers', () => {
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={readOnlyResourceServer}
        identifier={readOnlyResourceServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.getByLabelText(/Identifier/i)).toBeDisabled();
  });

  it('does not render inline Save or Discard buttons', () => {
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.queryByRole('button', {name: /Save/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Discard/i})).not.toBeInTheDocument();
  });

  it('renders the resource server copy for non-MCP resource servers', () => {
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Configuration settings for this resource server.')).toBeInTheDocument();
    expect(
      screen.getByText(
        'A unique value that identifies this resource server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
      ),
    ).toBeInTheDocument();
  });

  it('renders the MCP server copy for MCP resource servers', () => {
    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockMcpServer}
        identifier={mockMcpServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Configuration settings for this MCP server.')).toBeInTheDocument();
    expect(
      screen.getByText(
        'A unique value that identifies this MCP server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
      ),
    ).toBeInTheDocument();
  });

  it('selects an AuthZEN PDP connection as the authorization engine', () => {
    const onAuthorizationEngineChange = vi.fn();
    const onPDPConnectionChange = vi.fn();

    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        authorizationEngine={AuthorizationEngines.RBAC}
        pdpConnectionId=""
        onIdentifierChange={vi.fn()}
        onAuthorizationEngineChange={onAuthorizationEngineChange}
        onPDPConnectionChange={onPDPConnectionChange}
      />,
    );

    fireEvent.mouseDown(screen.getByRole('combobox', {name: 'Authorization engine'}));
    fireEvent.click(screen.getByText('AuthZEN PDP'));

    expect(onAuthorizationEngineChange).toHaveBeenCalledWith(AuthorizationEngines.AUTHZEN_PDP);
    expect(onPDPConnectionChange).toHaveBeenCalledWith('pdp-1');
  });

  it('links to connection creation when no PDP connections exist', () => {
    mockPDPConnections = {data: [], isLoading: false, error: null};

    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.getByText('No external PDP connections are available.')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Create a PDP connection'})).toHaveAttribute('href', '/connections/create');
  });

  it('marks the authorization engine field as invalid when PDP connections fail to load', () => {
    mockPDPConnections = {data: [], isLoading: false, error: new Error('Failed to load')};

    renderWithProviders(
      <AdvancedTab
        {...engineProps}
        resourceServer={mockResourceServer}
        identifier={mockResourceServer.identifier ?? ''}
        onIdentifierChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Authorization engine')).toHaveClass('Mui-error');
    expect(screen.getByText('Failed to load AuthZEN PDP connections.')).toHaveClass('Mui-error');
  });
});
