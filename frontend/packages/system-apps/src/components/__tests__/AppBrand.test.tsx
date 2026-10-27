// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ThemeProvider} from '@thunderid/react';
import {render, screen} from '@thunderid/test-utils';
import type {ReactElement, ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AppBrand from '../AppBrand';

const mockUser = vi.hoisted(() => ({
  claims: {ouHandle: 'default', ouName: 'Default'} as Record<string, unknown>,
  logo: undefined as string | undefined,
  meta: null as {application?: {logoUrl?: string}} | null,
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useConfig: () => ({config: {brand: {logo: mockUser.logo}}}),
}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  User: ({children}: {children: (user: Record<string, unknown>) => ReactNode}): ReactNode => children(mockUser.claims),
  useThunderID: () => ({meta: mockUser.meta}),
}));

// The SDK's Logo reads the SDK theme.
const renderBrand = (ui: ReactElement): ReturnType<typeof render> => render(<ThemeProvider>{ui}</ThemeProvider>);

describe('AppBrand', () => {
  beforeEach(() => {
    mockUser.logo = undefined;
    mockUser.meta = null;
  });

  it('shows the logo of the application it signs in with, from the flow metadata', () => {
    mockUser.meta = {application: {logoUrl: 'https://acme.example/app-logo.svg'}};

    renderBrand(<AppBrand name="Admin" config={{branding: {showSecuredBy: true}}} />);

    expect(screen.getByRole('img', {name: 'Admin'}).getAttribute('src')).toBe('https://acme.example/app-logo.svg');
  });

  it("prefers the logo in the app's runtime configuration over the application's", () => {
    mockUser.logo = 'https://acme.example/runtime-logo.svg';
    mockUser.meta = {application: {logoUrl: 'https://acme.example/app-logo.svg'}};

    renderBrand(
      <AppBrand
        name="Admin"
        config={{branding: {logoUrl: 'https://acme.example/admin-logo.svg', showSecuredBy: true}}}
      />,
    );

    expect(screen.getByRole('img', {name: 'Admin'}).getAttribute('src')).toBe('https://acme.example/runtime-logo.svg');
  });

  it("prefers the logo in the app's configuration over the application's", () => {
    mockUser.meta = {application: {logoUrl: 'https://acme.example/app-logo.svg'}};

    renderBrand(
      <AppBrand
        name="Admin"
        config={{branding: {logoUrl: 'https://acme.example/admin-logo.svg', showSecuredBy: true}}}
      />,
    );

    expect(screen.getByRole('img', {name: 'Admin'}).getAttribute('src')).toBe('https://acme.example/admin-logo.svg');
  });

  it('draws a generated avatar logo as its image, not as a broken avatar: URL', () => {
    mockUser.meta = {application: {logoUrl: 'avatar:shape=rounded,variant=two_letter,content=A,colors=0'}};

    renderBrand(<AppBrand name="Admin" config={{branding: {showSecuredBy: true}}} />);

    expect(screen.getByRole('img', {name: 'Admin'}).getAttribute('src')).toMatch(/^data:image\/svg\+xml/);
  });

  it('draws an emoji logo as its glyph', () => {
    mockUser.meta = {application: {logoUrl: 'emoji:🛡️'}};

    renderBrand(<AppBrand name="Admin" config={{branding: {showSecuredBy: true}}} />);

    expect(screen.getByRole('img', {name: 'Admin'}).textContent).toBe('🛡️');
  });

  it('shows no logo when none is set anywhere', () => {
    renderBrand(<AppBrand name="Admin" config={{branding: {showSecuredBy: true}}} />);

    expect(screen.queryByRole('img', {name: 'Admin'})).toBeNull();
  });

  it("shows the app's name, then the organization unit of the signed-in user", () => {
    renderBrand(<AppBrand name="Admin" config={{branding: {showSecuredBy: true}}} />);

    expect(screen.getByText('Admin')).toBeTruthy();
    expect(screen.getByLabelText('Organization unit').textContent).toBe('/DefaultOU');
  });

  it("shows the title in the app's configuration in place of the name", () => {
    renderBrand(<AppBrand name="Admin" config={{branding: {showSecuredBy: true, title: 'Acme'}}} />);

    expect(screen.getByText('Acme')).toBeTruthy();
    expect(screen.queryByText('Admin')).toBeNull();
  });

  it('leaves out the organization unit when the token names none', () => {
    mockUser.claims = {};

    renderBrand(<AppBrand name="Account" config={{branding: {showSecuredBy: true}}} />);

    expect(screen.getByText('Account')).toBeTruthy();
    expect(screen.queryByLabelText('Organization unit')).toBeNull();
  });
});
