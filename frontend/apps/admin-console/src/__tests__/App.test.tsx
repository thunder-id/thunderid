// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import type {JSX} from 'react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import App from '../App';

// The real ProtectedRoute runs; only the session state it reads is controlled.
const session = vi.hoisted(() => ({
  isLoading: false,
  isSignedIn: true,
  signIn: vi.fn(() => Promise.resolve()),
}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({
    isLoading: session.isLoading,
    isSignedIn: session.isSignedIn,
    signIn: session.signIn,
    signInOptions: {},
    signInUrl: undefined,
    tokenRequest: undefined,
  }),
}));

vi.mock('../layouts/AdminLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('../pages/ResourcePages', () => ({
  ResourceCreatePage: ({resource}: {resource: string}): JSX.Element => <p>{resource} create</p>,
  ResourceDetailsPage: ({resource}: {resource: string}): JSX.Element => <p>{resource} details</p>,
  ResourceListPage: ({resource}: {resource: string}): JSX.Element => <p>{resource} list</p>,
}));

const renderAt = (path: string): void => {
  window.history.pushState({}, '', path);
  render(<App />);
};

describe('App', () => {
  beforeEach(() => {
    session.isLoading = false;
    session.isSignedIn = true;
    session.signIn.mockClear();
  });

  afterEach(() => {
    window.history.pushState({}, '', '/');
  });

  it('keeps every page behind sign-in', async () => {
    session.isSignedIn = false;
    renderAt('/users');
    await waitFor(() => expect(session.signIn).toHaveBeenCalledTimes(1));
    expect(screen.queryByText('users list')).not.toBeInTheDocument();
  });

  // Without a loader, a protected page renders nothing while sign-in is checked or a redirect to
  // sign-in is on its way, which shows as a bare page mid sign-out.
  it('shows the page loader while sign-in is checked', () => {
    session.isLoading = true;
    renderAt('/users');
    expect(screen.getByRole('progressbar', {name: 'Loading content'})).toBeInTheDocument();
    expect(screen.queryByText('users list')).not.toBeInTheDocument();
    expect(session.signIn).not.toHaveBeenCalled();
  });

  it('shows the page loader while the redirect to sign-in is on its way', async () => {
    session.isSignedIn = false;
    renderAt('/users');
    expect(screen.getByRole('progressbar', {name: 'Loading content'})).toBeInTheDocument();
    await waitFor(() => expect(session.signIn).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('progressbar', {name: 'Loading content'})).toBeInTheDocument();
  });

  it('shows the page once signed in, without the loader', async () => {
    renderAt('/users');
    expect(await screen.findByText('users list')).toBeInTheDocument();
    expect(screen.queryByRole('progressbar', {name: 'Loading content'})).not.toBeInTheDocument();
  });

  it('redirects the root path to the users list', async () => {
    renderAt('/');
    expect(await screen.findByText('users list')).toBeInTheDocument();
  });

  it.each(['users', 'groups', 'roles', 'agents'])('mounts the list page of %s', async (resource) => {
    renderAt(`/${resource}`);
    expect(await screen.findByText(`${resource} list`)).toBeInTheDocument();
  });

  it('opens the create dialog route', async () => {
    renderAt('/groups/new');
    expect(await screen.findByText('groups create')).toBeInTheDocument();
  });

  it('opens a resource by its id', async () => {
    renderAt('/roles/r-1');
    expect(await screen.findByText('roles details')).toBeInTheDocument();
  });

  it('sends an unknown path to the users list', async () => {
    renderAt('/nowhere');
    expect(await screen.findByText('users list')).toBeInTheDocument();
  });
});
