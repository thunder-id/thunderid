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
import type {ResourceServer} from '../../../models/resource-server';
import AdvancedTab from '../AdvancedTab';

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
  });

  it('renders the Configurations section with the current identifier value', () => {
    renderWithProviders(
      <AdvancedTab
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
});
