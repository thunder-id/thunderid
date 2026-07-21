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

    expect(screen.getByText('Client type')).toBeInTheDocument();
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

    expect(screen.getByRole('radiogroup', {name: 'Client type'})).toBeInTheDocument();
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
    expect(userDelegatedCard).toHaveAttribute('aria-checked', 'true');
    expect(m2mCard).toHaveAttribute('aria-checked', 'false');
  });

  it('should mark the machine-to-machine card as selected when selectedType is m2m', () => {
    render(
      <ConfigureMcpClientType selectedType="m2m" onSelect={vi.fn()} redirectUris={[]} onRedirectUrisChange={vi.fn()} />,
    );

    const [userDelegatedCard, m2mCard] = screen.getAllByRole('radio');
    expect(userDelegatedCard).toHaveAttribute('aria-checked', 'false');
    expect(m2mCard).toHaveAttribute('aria-checked', 'true');
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

  it('should call onSelect when Enter is pressed on a card', () => {
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
    m2mCard.focus();
    m2mCard.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', bubbles: true, cancelable: true}));

    expect(onSelect).toHaveBeenCalledWith('m2m');
  });

  it('should call onSelect when Space is pressed on a card', () => {
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
    userDelegatedCard.focus();
    userDelegatedCard.dispatchEvent(new KeyboardEvent('keydown', {key: ' ', bubbles: true, cancelable: true}));

    expect(onSelect).toHaveBeenCalledWith('userDelegated');
  });

  it('should make each card keyboard-focusable', () => {
    render(
      <ConfigureMcpClientType
        selectedType="userDelegated"
        onSelect={vi.fn()}
        redirectUris={[]}
        onRedirectUrisChange={vi.fn()}
      />,
    );

    screen.getAllByRole('radio').forEach((card) => {
      expect(card).toHaveAttribute('tabIndex', '0');
    });
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
    it('should show the user-delegated OAuth profile chips and next-step line by default', () => {
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
      expect(screen.getByText('Add your redirect URIs below.')).toBeInTheDocument();
    });

    it('should show the machine-to-machine OAuth profile chips and next-step line when m2m is selected', () => {
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
      expect(screen.getByText('Next: your client ID and secret are generated.')).toBeInTheDocument();
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
