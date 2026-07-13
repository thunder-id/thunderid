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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OAuthAgentConfig} from '../../../../models/agent';
import RedirectURIsSection from '../RedirectURIsSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('RedirectURIsSection', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns null when oauth2Config is undefined', () => {
    const {container} = render(<RedirectURIsSection />);
    expect(container.firstChild).toBeNull();
  });

  it('returns null when no redirect-using grant is selected', () => {
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['client_credentials'],
      responseTypes: [],
    };

    const {container} = render(<RedirectURIsSection oauth2Config={oauth2Config} />);

    expect(container.firstChild).toBeNull();
  });

  it('renders the section when authorization_code grant is selected', () => {
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['https://example.com/cb'],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    expect(screen.getByText('Authorized redirect URIs')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://example.com/cb')).toBeInTheDocument();
  });

  it('shows the missing-redirect-uri error when no valid URI is configured', () => {
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: [],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    expect(screen.getByTestId('agent-redirect-uris-required')).toBeInTheDocument();
  });

  it('shows no error when at least one valid URI is configured', () => {
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['https://example.com/cb'],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    expect(screen.queryByTestId('agent-redirect-uris-required')).not.toBeInTheDocument();
  });

  it('treats blank entries as invalid', () => {
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['  '],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    expect(screen.getByTestId('agent-redirect-uris-required')).toBeInTheDocument();
  });

  it('appends a new URI when "Add URI" is clicked', async () => {
    const user = userEvent.setup();
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['https://a.example.com'],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    await user.click(screen.getByRole('button', {name: /Add URI/i}));

    expect(mockOnChange).toHaveBeenCalledWith({redirectUris: ['https://a.example.com', '']});
  });

  it('removes a URI when its delete button is clicked', async () => {
    const user = userEvent.setup();
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['https://a.example.com', 'https://b.example.com'],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    const deleteButtons = screen.getAllByRole('button', {name: /Delete/i});
    await user.click(deleteButtons[0]);

    expect(mockOnChange).toHaveBeenCalledWith({redirectUris: ['https://b.example.com']});
  });

  it('shifts error indices down after removing an earlier URI', async () => {
    const user = userEvent.setup();
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['', '', ''],
    };

    const {container} = render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    const inputs = container.querySelectorAll('input');
    await user.click(inputs[0]);
    await user.tab();
    await user.click(inputs[2]);
    await user.tab();

    expect(screen.getAllByText('URI cannot be empty')).toHaveLength(2);

    const deleteButtons = screen.getAllByRole('button', {name: /Delete/i});
    await user.click(deleteButtons[1]);

    expect(container.querySelector('#agent-redirect-uri-0')).toHaveAttribute('aria-invalid', 'true');
    expect(container.querySelector('#agent-redirect-uri-1')).toHaveAttribute('aria-invalid', 'true');
    expect(container.querySelector('#agent-redirect-uri-2')).not.toHaveAttribute('aria-invalid', 'true');
  });

  it('updates a URI when typed', async () => {
    const user = userEvent.setup({delay: null});
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: [''],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    const input = screen.getByPlaceholderText('https://example.com/callback');
    await user.type(input, 'h');

    // commit() is called for each keystroke
    expect(mockOnChange).toHaveBeenCalledWith({redirectUris: ['h']});
  });

  it('shows an inline empty error after blurring an empty field', async () => {
    const user = userEvent.setup();
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: [''],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    const input = screen.getByPlaceholderText('https://example.com/callback');
    await user.click(input);
    await user.tab();

    expect(screen.getByText('URI cannot be empty')).toBeInTheDocument();
  });

  it('shows an inline invalid-URL error for malformed entries', async () => {
    const user = userEvent.setup();
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['not a url'],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} onOAuth2ConfigChange={mockOnChange} />);

    const input = screen.getByPlaceholderText('https://example.com/callback');
    await user.click(input);
    await user.tab();

    expect(screen.getByText('Enter a valid URL')).toBeInTheDocument();
  });

  it('hides the add button and delete buttons when not editable', () => {
    const oauth2Config: OAuthAgentConfig = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      redirectUris: ['https://example.com'],
    };

    render(<RedirectURIsSection oauth2Config={oauth2Config} />);

    expect(screen.queryByRole('button', {name: /Add URI/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Delete/i})).not.toBeInTheDocument();
  });
});
