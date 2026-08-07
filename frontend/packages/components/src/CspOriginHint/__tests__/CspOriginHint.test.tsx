// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import CspOriginHint from '../CspOriginHint';
import {resolveCspHint} from '../resolveCspHint';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, _defaultValue?: string, opts?: {origin?: string}): string => {
      if (key === 'common:learnMore') {
        return 'Learn more';
      }
      return opts?.origin ? `${key}::${opts.origin}` : key;
    },
  }),
}));

const {mockGetDocumentationLink} = vi.hoisted(() => ({
  mockGetDocumentationLink: vi.fn<(key: string) => string | undefined>(
    () => 'https://thunderid.dev/docs/deployment/configuration/#content-security-policy',
  ),
}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({getDocumentationLink: mockGetDocumentationLink}),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn(), warn: vi.fn()}),
}));

describe('resolveCspHint', () => {
  it('returns null for a relative path', () => {
    expect(resolveCspHint('assets/logo.png', 'image')).toBeNull();
  });

  it('returns null for a data URI', () => {
    expect(resolveCspHint('data:image/png;base64,AAAA', 'image')).toBeNull();
  });

  it('returns null for emoji and avatar sentinels', () => {
    expect(resolveCspHint('emoji:1f600', 'image')).toBeNull();
    expect(resolveCspHint('avatar:gradient-1', 'image')).toBeNull();
  });

  it('returns null for a blank value', () => {
    expect(resolveCspHint('   ', 'image')).toBeNull();
  });

  it('returns null for a same-origin URL', () => {
    expect(resolveCspHint(`${window.location.origin}/logo.png`, 'image')).toBeNull();
  });

  it('maps an external image URL to img-src guidance', () => {
    expect(resolveCspHint('https://cdn.example.com/logo.png', 'image')).toEqual({
      origin: 'https://cdn.example.com',
      messageKey: 'common:csp.hint.image',
    });
  });

  it('maps an external stylesheet URL to style-src-elem guidance', () => {
    expect(resolveCspHint('https://cdn.example.com/theme.css', 'stylesheet')).toEqual({
      origin: 'https://cdn.example.com',
      messageKey: 'common:csp.hint.stylesheet',
    });
  });

  it('maps a Google Fonts stylesheet URL to the two-directive guidance', () => {
    expect(resolveCspHint('https://fonts.googleapis.com/css2?family=Poppins', 'font')).toEqual({
      origin: 'https://fonts.googleapis.com',
      messageKey: 'common:csp.hint.fontGoogle',
    });
  });

  it('maps an arbitrary font URL to font-src guidance', () => {
    expect(resolveCspHint('https://fonts.example.com/font.css', 'font')).toEqual({
      origin: 'https://fonts.example.com',
      messageKey: 'common:csp.hint.font',
    });
  });

  it('preserves a non-standard port in the origin', () => {
    expect(resolveCspHint('https://cdn.example.com:8443/logo.png', 'image')).toEqual({
      origin: 'https://cdn.example.com:8443',
      messageKey: 'common:csp.hint.image',
    });
  });
});

describe('CspOriginHint', () => {
  it('renders nothing when the value is not an external URL', () => {
    const {container} = render(<CspOriginHint value="assets/logo.png" resourceType="image" />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders the directive guidance and a docs link for an external URL', () => {
    render(<CspOriginHint value="https://cdn.example.com/logo.png" resourceType="image" />);

    expect(screen.getByRole('alert')).toHaveTextContent('common:csp.hint.image::https://cdn.example.com');
    expect(screen.getByText('Learn more')).toBeInTheDocument();
  });
});
