// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';
import ConfigureMcpConnection from '../ConfigureMcpConnection';

describe('ConfigureMcpConnection', () => {
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

    expect(
      screen.getByText('Enter a valid HTTPS URI or an HTTP URI using localhost, 127.0.0.1, or [::1].'),
    ).toBeInTheDocument();
  });

  it('should clear the inline error once the value becomes valid', async () => {
    const user = userEvent.setup();
    render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

    const input = screen.getByPlaceholderText('http://localhost:8080/callback');
    await user.type(input, 'http://example.com/cb');
    await user.tab();
    expect(
      screen.getByText('Enter a valid HTTPS URI or an HTTP URI using localhost, 127.0.0.1, or [::1].'),
    ).toBeInTheDocument();

    await user.clear(input);
    await user.type(input, 'https://agent.example.com/cb');

    expect(
      screen.queryByText('Enter a valid HTTPS URI or an HTTP URI using localhost, 127.0.0.1, or [::1].'),
    ).not.toBeInTheDocument();
  });

  describe('MCP Inspector guidance', () => {
    it('should render the Inspector hint above the first redirect input', () => {
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

      expect(screen.getByText('Testing with MCP Inspector? Use', {exact: false})).toBeInTheDocument();
      expect(screen.getByText('http://localhost:6274/oauth/callback')).toBeInTheDocument();
    });

    it('should not render a suggestion chip', () => {
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={vi.fn()} />);

      expect(screen.queryByText('Suggested:')).not.toBeInTheDocument();
    });

    it('should fill the first empty redirect URI row when "Add it to redirect URIs" is clicked', async () => {
      const user = userEvent.setup();
      const onRedirectUrisChange = vi.fn();
      render(<ConfigureMcpConnection redirectUris={[]} onRedirectUrisChange={onRedirectUrisChange} />);

      await user.click(screen.getByText('Add it to redirect URIs'));

      expect(onRedirectUrisChange).toHaveBeenLastCalledWith(['http://localhost:6274/oauth/callback']);
    });

    it('should append a new row when every existing row is already filled', async () => {
      const user = userEvent.setup();
      const onRedirectUrisChange = vi.fn();
      render(
        <ConfigureMcpConnection
          redirectUris={['https://agent.example.com/cb']}
          onRedirectUrisChange={onRedirectUrisChange}
        />,
      );

      await user.click(screen.getByText('Add it to redirect URIs'));

      expect(onRedirectUrisChange).toHaveBeenLastCalledWith([
        'https://agent.example.com/cb',
        'http://localhost:6274/oauth/callback',
      ]);
    });

    it('should not add a duplicate row when the Inspector URI is already present', async () => {
      const user = userEvent.setup();
      const onRedirectUrisChange = vi.fn();
      render(
        <ConfigureMcpConnection
          redirectUris={['http://localhost:6274/oauth/callback']}
          onRedirectUrisChange={onRedirectUrisChange}
        />,
      );

      await user.click(screen.getByText('Add it to redirect URIs'));

      expect(onRedirectUrisChange).not.toHaveBeenCalled();
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
