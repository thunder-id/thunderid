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

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import ConfigureName from '../ConfigureName';

vi.mock('@thunderid/utils');

const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

const mockSuggestions = ['Alpha Service', 'Beta Platform', 'Gamma API', 'Delta Hub', 'Epsilon Suite'];

describe('ConfigureName', () => {
  beforeEach(() => {
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue(mockSuggestions);
  });

  it('renders the name and identifier input fields', () => {
    render(<ConfigureName name="" identifier="" onNameChange={vi.fn()} onIdentifierChange={vi.fn()} />);

    expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    expect(screen.getByRole('textbox', {name: /identifier/i})).toBeInTheDocument();
  });

  it('calls onNameChange when name input changes', () => {
    const onNameChange = vi.fn();
    const onIdentifierChange = vi.fn();
    render(<ConfigureName name="" identifier="" onNameChange={onNameChange} onIdentifierChange={onIdentifierChange} />);

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });

    expect(onNameChange).toHaveBeenCalledWith('Payments API');
    expect(onIdentifierChange).not.toHaveBeenCalled();
  });

  it('calls onIdentifierChange when the identifier input changes', () => {
    const onIdentifierChange = vi.fn();
    render(<ConfigureName name="Test" identifier="" onNameChange={vi.fn()} onIdentifierChange={onIdentifierChange} />);

    fireEvent.change(screen.getByRole('textbox', {name: /identifier/i}), {
      target: {value: 'https://api.example.com'},
    });

    expect(onIdentifierChange).toHaveBeenCalledWith('https://api.example.com');
  });

  it('calls onReadyChange with true when name and identifier are non-empty', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureName
        name="Test"
        identifier="https://api.example.com"
        onNameChange={vi.fn()}
        onIdentifierChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('calls onReadyChange with false when name is empty', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureName
        name=""
        identifier="https://api.example.com"
        onNameChange={vi.fn()}
        onIdentifierChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(false);
  });

  it('calls onReadyChange with false when identifier is empty', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureName
        name="Test"
        identifier=""
        onNameChange={vi.fn()}
        onIdentifierChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(false);
  });

  it('renders suggestion chips from the returned suggestions', () => {
    render(<ConfigureName name="" identifier="" onNameChange={vi.fn()} onIdentifierChange={vi.fn()} />);

    expect(screen.getByText('Alpha Service')).toBeInTheDocument();
    expect(screen.getByText('Beta Platform')).toBeInTheDocument();
  });

  it('fills name when a suggestion chip is clicked', () => {
    const onNameChange = vi.fn();
    const onIdentifierChange = vi.fn();
    render(<ConfigureName name="" identifier="" onNameChange={onNameChange} onIdentifierChange={onIdentifierChange} />);

    fireEvent.click(screen.getByText('Alpha Service'));

    expect(onNameChange).toHaveBeenCalledWith('Alpha Service');
    expect(onIdentifierChange).not.toHaveBeenCalled();
  });

  it('renders the resource server title and label when selectedType is not MCP', () => {
    render(
      <ConfigureName name="" identifier="" selectedType="API" onNameChange={vi.fn()} onIdentifierChange={vi.fn()} />,
    );

    expect(screen.getByText('Name your resource server')).toBeInTheDocument();
    expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
  });

  it('renders the MCP server title and label when selectedType is MCP', () => {
    render(
      <ConfigureName name="" identifier="" selectedType="MCP" onNameChange={vi.fn()} onIdentifierChange={vi.fn()} />,
    );

    expect(screen.getByText('Name your MCP server')).toBeInTheDocument();
    expect(screen.getByRole('textbox', {name: /mcp server name/i})).toBeInTheDocument();
  });

  it('renders the MCP identifier helper text when selectedType is MCP', () => {
    render(
      <ConfigureName name="" identifier="" selectedType="MCP" onNameChange={vi.fn()} onIdentifierChange={vi.fn()} />,
    );

    expect(
      screen.getByText(
        'A unique identifier for this MCP server. When set as an absolute URI, it becomes the token audience for RFC 8707 resource indicators.',
      ),
    ).toBeInTheDocument();
  });
});
