// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {JSX} from 'react';
import {MemoryRouter, Route, Routes, useLocation} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {HomePage, ProfilePage, SecurityPage} from '../pages/AccountPages';

const mockPages: Record<string, {actions: Record<string, boolean>; enabled: boolean}> = {};
const mockState = {isLoading: false};

vi.mock('../hooks/useAccountAppConfig', () => ({
  default: () => {
    const page = (key: string) => mockPages[key];

    return {
      isLoading: mockState.isLoading,
      page,
      visiblePages: ['profile', 'security'].filter((key) => page(key)?.enabled),
    };
  },
}));

vi.mock('@thunderid/react', () => ({
  ChangeCredential: (): JSX.Element => <p>password form</p>,
  User: ({children}: {children: (user: Record<string, unknown>) => JSX.Element}): JSX.Element =>
    children({given_name: 'Aiko', name: 'Aiko Tanaka'}),
  UserProfile: ({editable}: {editable: boolean}): JSX.Element => <p>profile {editable ? 'editable' : 'read only'}</p>,
}));

function CurrentPath(): JSX.Element {
  return <output>{useLocation().pathname}</output>;
}

const renderAt = (path: string): void => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/security" element={<SecurityPage />} />
      </Routes>
      <CurrentPath />
    </MemoryRouter>,
  );
};

describe('Account App pages', () => {
  beforeEach(() => {
    mockState.isLoading = false;
    mockPages.profile = {actions: {edit: true}, enabled: true};
    mockPages.security = {actions: {password: true}, enabled: true};
  });

  it('lets the user edit their profile when the Console allows it', () => {
    renderAt('/profile');

    expect(screen.getByText('profile editable')).toBeTruthy();
  });

  it('keeps the profile read only when editing is off', () => {
    mockPages.profile.actions = {edit: false};

    renderAt('/profile');

    expect(screen.getByText('profile read only')).toBeTruthy();
  });

  it('greets the user and links to each page that is on', () => {
    mockPages.security.enabled = false;

    renderAt('/');

    expect(screen.getByRole('heading', {name: 'Welcome, Aiko Tanaka'})).toBeTruthy();
    expect(screen.getByRole('link', {name: 'Manage your personal info'}).getAttribute('href')).toBe('/profile');
    expect(screen.queryByRole('link', {name: 'Manage your security'})).toBeNull();
  });

  it('shows the password form open on the security page', () => {
    renderAt('/security');

    expect(screen.getByRole('heading', {name: 'Password'})).toBeTruthy();
    expect(screen.getByText('password form')).toBeTruthy();
  });

  it('offers no password change when it is off', () => {
    mockPages.security.actions = {password: false};

    renderAt('/security');

    expect(screen.queryByText('password form')).toBeNull();
    expect(screen.getByText('There is nothing you can change here yet.')).toBeTruthy();
  });

  it('sends a page the Console turned off back home', () => {
    mockPages.profile.enabled = false;

    renderAt('/profile');

    expect(screen.getByRole('status').textContent).toBe('/');
  });
});
