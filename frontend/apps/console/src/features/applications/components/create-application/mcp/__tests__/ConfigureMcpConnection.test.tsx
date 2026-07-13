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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConfigureMcpConnection from '../ConfigureMcpConnection';

const mockCopy = vi.fn().mockResolvedValue(undefined);

vi.mock('@thunderid/hooks', () => ({
  useCopyToClipboard: vi.fn(() => ({copied: false, copy: mockCopy})),
}));

describe('ConfigureMcpConnection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCopy.mockResolvedValue(undefined);
  });

  it('should render the title and subtitle', () => {
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

    expect(screen.getByText('Add a redirect URI')).toBeInTheDocument();
    expect(screen.getByText('Where should users be sent after they authorize this client?')).toBeInTheDocument();
  });

  it('should hide the title and subtitle when compact is true', () => {
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} compact />);

    expect(screen.queryByText('Add a redirect URI')).not.toBeInTheDocument();
    expect(screen.queryByText('Where should users be sent after they authorize this client?')).not.toBeInTheDocument();
  });

  it('should render a single empty redirect URI row by default', () => {
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

    expect(screen.getAllByPlaceholderText('http://localhost:8080/callback')).toHaveLength(1);
  });

  it('should render a row for each existing redirect URI', () => {
    render(
      <ConfigureMcpConnection
        redirectUris={['https://agent.example.com/cb', 'http://localhost:3000/cb']}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    expect(screen.getByDisplayValue('https://agent.example.com/cb')).toBeInTheDocument();
    expect(screen.getByDisplayValue('http://localhost:3000/cb')).toBeInTheDocument();
  });

  it('should add a new empty row when the add button is clicked', async () => {
    const user = userEvent.setup();
    const onRedirectUrisChange = vi.fn();
    render(
      <ConfigureMcpConnection
        redirectUris={['https://agent.example.com/cb']}
        onRedirectUrisChange={onRedirectUrisChange}
      />,
    );

    await user.click(screen.getByRole('button', {name: 'Add redirect URI'}));

    expect(screen.getAllByPlaceholderText('http://localhost:8080/callback')).toHaveLength(2);
    expect(onRedirectUrisChange).toHaveBeenLastCalledWith(['https://agent.example.com/cb', '']);
  });

  it('should remove a row when its delete button is clicked', async () => {
    const user = userEvent.setup();
    const onRedirectUrisChange = vi.fn();
    render(
      <ConfigureMcpConnection
        redirectUris={['https://agent.example.com/cb', 'http://localhost:3000/cb']}
        onRedirectUrisChange={onRedirectUrisChange}
      />,
    );

    const [firstRemoveButton] = screen.getAllByLabelText('Remove redirect URI');
    await user.click(firstRemoveButton);

    expect(screen.queryByDisplayValue('https://agent.example.com/cb')).not.toBeInTheDocument();
    expect(onRedirectUrisChange).toHaveBeenLastCalledWith(['http://localhost:3000/cb']);
  });

  it('should keep a single empty row when the last redirect URI is removed', async () => {
    const user = userEvent.setup();
    const onRedirectUrisChange = vi.fn();
    render(
      <ConfigureMcpConnection
        redirectUris={['https://agent.example.com/cb']}
        onRedirectUrisChange={onRedirectUrisChange}
      />,
    );

    await user.click(screen.getByLabelText('Remove redirect URI'));

    expect(screen.getAllByPlaceholderText('http://localhost:8080/callback')).toHaveLength(1);
    expect(onRedirectUrisChange).toHaveBeenLastCalledWith(['']);
  });

  it('should edit a row value and propagate the change', async () => {
    const user = userEvent.setup();
    const onRedirectUrisChange = vi.fn();
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={onRedirectUrisChange} />);

    const input = screen.getByPlaceholderText('http://localhost:8080/callback');
    await user.type(input, 'https://agent.example.com/cb');

    expect(onRedirectUrisChange).toHaveBeenLastCalledWith(['https://agent.example.com/cb']);
  });

  it('should trim whitespace from the persisted value while typing', async () => {
    const user = userEvent.setup();
    const onRedirectUrisChange = vi.fn();
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={onRedirectUrisChange} />);

    const input = screen.getByPlaceholderText('http://localhost:8080/callback');
    await user.type(input, 'https://agent.example.com/cb ');

    expect(onRedirectUrisChange).toHaveBeenLastCalledWith(['https://agent.example.com/cb']);
  });

  it('should show an inline error when an invalid URI is blurred', async () => {
    const user = userEvent.setup();
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

    const input = screen.getByPlaceholderText('http://localhost:8080/callback');
    await user.type(input, 'http://example.com/cb');
    await user.tab();

    expect(screen.getByText('Enter a valid loopback (http://127.0.0.1) or HTTPS URI.')).toBeInTheDocument();
  });

  it('should clear the inline error once the value becomes valid', async () => {
    const user = userEvent.setup();
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

    const input = screen.getByPlaceholderText('http://localhost:8080/callback');
    await user.type(input, 'http://example.com/cb');
    await user.tab();
    expect(screen.getByText('Enter a valid loopback (http://127.0.0.1) or HTTPS URI.')).toBeInTheDocument();

    await user.clear(input);
    await user.type(input, 'https://agent.example.com/cb');

    expect(screen.queryByText('Enter a valid loopback (http://127.0.0.1) or HTTPS URI.')).not.toBeInTheDocument();
  });

  describe('MCP Inspector guidance', () => {
    it('should render the Inspector hint above the first redirect input', () => {
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

      expect(
        screen.getByText('Testing with MCP Inspector? Use http://localhost:6274/oauth/callback'),
      ).toBeInTheDocument();
    });

    it('should not render a suggestion chip', () => {
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

      expect(screen.queryByText('Suggested:')).not.toBeInTheDocument();
    });

    it('should copy the MCP Inspector callback URI when the copy button is clicked', async () => {
      const user = userEvent.setup();
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

      await user.click(screen.getByRole('button', {name: 'Copy MCP Inspector callback URI'}));

      expect(mockCopy).toHaveBeenCalledWith('http://localhost:6274/oauth/callback');
    });

    it('should not fill any redirect URI input when the copy button is clicked', async () => {
      const user = userEvent.setup();
      const onRedirectUrisChange = vi.fn();
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={onRedirectUrisChange} />);

      await user.click(screen.getByRole('button', {name: 'Copy MCP Inspector callback URI'}));

      expect(onRedirectUrisChange).not.toHaveBeenCalled();
      expect(screen.queryByDisplayValue('http://localhost:6274/oauth/callback')).not.toBeInTheDocument();
    });
  });

  describe('readiness', () => {
    it('should report not ready when there are no redirect URIs', () => {
      const onReadyChange = vi.fn();
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} onReadyChange={onReadyChange} />);

      expect(onReadyChange).toHaveBeenLastCalledWith(false);
    });

    it('should report ready once a valid redirect URI is entered', async () => {
      const user = userEvent.setup();
      const onReadyChange = vi.fn();
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} onReadyChange={onReadyChange} />);

      const input = screen.getByPlaceholderText('http://localhost:8080/callback');
      await user.type(input, 'https://agent.example.com/cb');

      expect(onReadyChange).toHaveBeenLastCalledWith(true);
    });

    it('should report not ready when the only redirect URI is invalid', async () => {
      const user = userEvent.setup();
      const onReadyChange = vi.fn();
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} onReadyChange={onReadyChange} />);

      const input = screen.getByPlaceholderText('http://localhost:8080/callback');
      await user.type(input, 'http://example.com/cb');

      expect(onReadyChange).toHaveBeenLastCalledWith(false);
    });

    it('should report ready with an already-populated valid redirect URI', () => {
      const onReadyChange = vi.fn();
      render(
        <ConfigureMcpConnection
          redirectUris={['https://agent.example.com/cb']}
          onRedirectUrisChange={vi.fn()}
          onReadyChange={onReadyChange}
        />,
      );

      expect(onReadyChange).toHaveBeenLastCalledWith(true);
    });

    it('should report not ready when one of multiple redirect URIs is invalid', () => {
      const onReadyChange = vi.fn();
      render(
        <ConfigureMcpConnection
          redirectUris={['https://agent.example.com/cb', 'http://example.com/cb']}
          onRedirectUrisChange={vi.fn()}
          onReadyChange={onReadyChange}
        />,
      );

      expect(onReadyChange).toHaveBeenLastCalledWith(false);
    });
  });
});
