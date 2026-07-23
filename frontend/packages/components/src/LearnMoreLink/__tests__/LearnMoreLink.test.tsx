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

/* eslint-disable @typescript-eslint/unbound-method */
import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import LearnMoreLink from '../LearnMoreLink';

const {mockGetDocumentationLink} = vi.hoisted(() => ({
  mockGetDocumentationLink: vi.fn<(key: string) => string | undefined>(),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string): string => {
      const translations: Record<string, string> = {
        'common:learnMore': 'Learn more',
        'common:externalLink.title': 'You are leaving ThunderID',
        'common:actions.stay': 'Stay',
        'common:actions.continue': 'Continue',
        'common:actions.close': 'Close',
        'common:actions.copy': 'Copy',
        'common:actions.copied': 'Copied!',
      };
      return translations[key] ?? key;
    },
  }),
}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    config: {brand: {product_name: 'ThunderID'}},
    getDocumentationLink: mockGetDocumentationLink,
  }),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn(), warn: vi.fn()}),
}));

Object.defineProperty(navigator, 'clipboard', {
  value: {writeText: vi.fn().mockResolvedValue(undefined)},
  configurable: true,
  writable: true,
});

describe('LearnMoreLink', () => {
  beforeEach(() => {
    mockGetDocumentationLink.mockReset();
    vi.spyOn(window, 'open').mockImplementation(() => null);
  });

  afterEach(() => {
    vi.mocked(window.open).mockRestore();
  });

  it('renders nothing when no link is configured for the key', () => {
    mockGetDocumentationLink.mockReturnValue(undefined);
    const {container} = render(<LearnMoreLink docKey="roles" />);

    expect(container).toBeEmptyDOMElement();
  });

  it('opens the confirm dialog instead of navigating immediately on click', () => {
    mockGetDocumentationLink.mockReturnValue('https://thunderid.dev/docs/next/guides/users');
    render(<LearnMoreLink docKey="users" />);

    fireEvent.click(screen.getByText('Learn more'));

    expect(screen.getByText('You are leaving ThunderID')).toBeInTheDocument();
    expect(window.open).not.toHaveBeenCalled();
  });

  it('navigates to the resolved URL when Continue is clicked', () => {
    mockGetDocumentationLink.mockReturnValue('https://thunderid.dev/docs/next/guides/users');
    render(<LearnMoreLink docKey="users" />);

    fireEvent.click(screen.getByText('Learn more'));
    fireEvent.click(screen.getByText('Continue'));

    expect(window.open).toHaveBeenCalledWith(
      'https://thunderid.dev/docs/next/guides/users',
      '_blank',
      'noopener,noreferrer',
    );
  });

  it('closes without navigating when Stay is clicked', async () => {
    mockGetDocumentationLink.mockReturnValue('https://thunderid.dev/docs/next/guides/users');
    render(<LearnMoreLink docKey="users" />);

    fireEvent.click(screen.getByText('Learn more'));
    fireEvent.click(screen.getByText('Stay'));

    expect(window.open).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByText('You are leaving ThunderID')).not.toBeInTheDocument();
    });
  });

  it('shows the destination hostname and a copyable URL field in the confirm dialog', () => {
    mockGetDocumentationLink.mockReturnValue('https://thunderid.dev/docs/next/guides/users');
    render(<LearnMoreLink docKey="users" />);

    fireEvent.click(screen.getByText('Learn more'));

    expect(screen.getByText('thunderid.dev')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://thunderid.dev/docs/next/guides/users')).toBeInTheDocument();
  });

  it('copies the URL to the clipboard when the copy button is clicked', async () => {
    mockGetDocumentationLink.mockReturnValue('https://thunderid.dev/docs/next/guides/users');
    render(<LearnMoreLink docKey="users" />);

    fireEvent.click(screen.getByText('Learn more'));
    fireEvent.click(screen.getByRole('button', {name: 'Copy'}));

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith('https://thunderid.dev/docs/next/guides/users');
    });
  });

  it('renders a custom label when provided', () => {
    mockGetDocumentationLink.mockReturnValue('https://thunderid.dev/docs/next/guides/applications');
    render(<LearnMoreLink docKey="applications" label="Read the guide" />);

    expect(screen.getByText('Read the guide')).toBeInTheDocument();
  });
});
