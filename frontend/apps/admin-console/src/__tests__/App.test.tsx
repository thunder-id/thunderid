// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import App from '../App';

vi.mock('@thunderid/react-router', () => ({
  ProtectedRoute: ({children}: {children: ReactNode}) => <div data-testid="protected-route">{children}</div>,
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
  afterEach(() => {
    window.history.pushState({}, '', '/');
  });

  it('keeps every page behind sign-in', () => {
    renderAt('/users');
    expect(screen.getByTestId('protected-route')).toBeInTheDocument();
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
