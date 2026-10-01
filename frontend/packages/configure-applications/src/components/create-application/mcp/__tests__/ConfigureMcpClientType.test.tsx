// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import ConfigureMcpClientType from '../ConfigureMcpClientType';

vi.mock('../ConfigureMcpConnection', () => ({
  default: ({compact, onReadyChange}: {compact?: boolean; onReadyChange?: (ready: boolean) => void}) => (
    <div data-testid="mcp-connection-section" data-compact={compact}>
      <button type="button" onClick={() => onReadyChange?.(true)} data-testid="mock-ready-trigger">
        Ready
      </button>
    </div>
  ),
}));

describe('ConfigureMcpClientType', () => {
  it('should render both client type cards', () => {
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    expect(screen.getAllByRole('radio')).toHaveLength(2);
  });

  it('should render the client type title and subtitle', () => {
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Client Type')).toBeInTheDocument();
    expect(screen.getByText('How will this client obtain tokens?')).toBeInTheDocument();
  });

  it('should render the user-delegated and machine-to-machine card copy', () => {
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    expect(screen.getByText('On behalf of a user')).toBeInTheDocument();
    expect(
      screen.getByText(
        'A client in a host app (IDE, desktop app, or chat client) that acts on behalf of a signed-in user. Uses Authorization Code with PKCE.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('On its own behalf')).toBeInTheDocument();
    expect(
      screen.getByText(
        'A client that authenticates with its own credentials without user interaction. Uses Client Credentials.',
      ),
    ).toBeInTheDocument();
  });

  it('should wrap the cards in an accessible radiogroup', () => {
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    expect(screen.getByRole('radiogroup', {name: 'Client Type'})).toBeInTheDocument();
  });

  it('should default-select the user-delegated card', () => {
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    const [userDelegatedCard, m2mCard] = screen.getAllByRole('radio');
    expect(userDelegatedCard).toBeChecked();
    expect(m2mCard).not.toBeChecked();
  });

  it('should mark the machine-to-machine card as selected when selectedType is m2m', () => {
    render(
      <ConfigureMcpClientType selectedType="m2m" onSelect={vi.fn()} redirectUris={[]} onRedirectUrisChange={vi.fn()} />,
    );

    const [userDelegatedCard, m2mCard] = screen.getAllByRole('radio');
    expect(userDelegatedCard).not.toBeChecked();
    expect(m2mCard).toBeChecked();
  });

  it('should call onSelect with "userDelegated" when the user-delegated card is clicked', () => {
    const onSelect = vi.fn();
    render(
      <ConfigureMcpClientType
        selectedType="m2m"
        onSelect={onSelect}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    const [userDelegatedCard] = screen.getAllByRole('radio');
    userDelegatedCard.click();

    expect(onSelect).toHaveBeenCalledWith('userDelegated');
  });

  it('should call onSelect with "m2m" when the machine-to-machine card is clicked', () => {
    const onSelect = vi.fn();
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={onSelect}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    const [, m2mCard] = screen.getAllByRole('radio');
    m2mCard.click();

    expect(onSelect).toHaveBeenCalledWith('m2m');
  });

  it('should render the redirect URI section when userDelegated is selected', () => {
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    expect(screen.getByTestId('mcp-connection-section')).toBeInTheDocument();
  });

  it('should hide the redirect URI section when m2m is selected', () => {
    render(
      <ConfigureMcpClientType selectedType="m2m" onSelect={vi.fn()} redirectUris={[]} onRedirectUrisChange={vi.fn()} />,
    );

    expect(screen.queryByTestId('mcp-connection-section')).not.toBeInTheDocument();
  });

  it('should call onReadyChange(true) when m2m is selected', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureMcpClientType
        selectedType="m2m"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  describe('preview panel', () => {
    it('should show the user-delegated OAuth profile chips by default', () => {
      render(
        <ConfigureMcpClientType
          selectedType="userDelegated"
          onSelect={vi.fn()}
          redirectUris={[]}
          onRedirectUrisChange={vi.fn()}
        />,
      );

      expect(screen.getByText('What you get')).toBeInTheDocument();
      expect(screen.getByText('Authorization Code + PKCE (required)')).toBeInTheDocument();
      expect(screen.getByText('Public client')).toBeInTheDocument();
      expect(screen.getByText('Refresh tokens')).toBeInTheDocument();
    });

    it('should show the machine-to-machine OAuth profile chips when m2m is selected', () => {
      render(
        <ConfigureMcpClientType
          selectedType="m2m"
          onSelect={vi.fn()}
          redirectUris={[]}
          onRedirectUrisChange={vi.fn()}
        />,
      );

      expect(screen.getByText('Client Credentials')).toBeInTheDocument();
      expect(screen.getByText('Confidential client')).toBeInTheDocument();
      expect(screen.getByText('Client secret issued')).toBeInTheDocument();
    });

    it('should swap the preview content when the selected type changes', () => {
      const {rerender} = render(
        <ConfigureMcpClientType
          selectedType="userDelegated"
          onSelect={vi.fn()}
          redirectUris={[]}
          onRedirectUrisChange={vi.fn()}
        />,
      );

      expect(screen.getByText('Public client')).toBeInTheDocument();
      expect(screen.queryByText('Confidential client')).not.toBeInTheDocument();

      rerender(
        <ConfigureMcpClientType
          selectedType="m2m"
          onSelect={vi.fn()}
          redirectUris={[]}
          onRedirectUrisChange={vi.fn()}
        />,
      );

      expect(screen.queryByText('Public client')).not.toBeInTheDocument();
      expect(screen.getByText('Confidential client')).toBeInTheDocument();
    });
  });
});
