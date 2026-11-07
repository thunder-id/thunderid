// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider, useQuery} from '@tanstack/react-query';
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SessionExpiredBanner from '../SessionExpiredBanner';

const signIn = vi.fn(() => Promise.resolve());
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({signIn}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => 'https://cp.test'})};
});

const refused = (url: string, status = 401): Error =>
  Object.assign(new Error('Request failed'), {response: {status}, config: {url}});

function Failing({error}: {error: Error}): ReactNode {
  useQuery({queryKey: ['failing', error.message, String(error)], queryFn: () => Promise.reject(error), retry: false});
  return null;
}

function renderWith(error: Error): void {
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <SessionExpiredBanner />
      <Failing error={error} />
    </QueryClientProvider>,
  );
}

describe('SessionExpiredBanner', () => {
  beforeEach(() => signIn.mockClear());

  it('says the session ended when another service refuses a call, and signs in again on request', async () => {
    renderWith(refused('https://org-mgt.test/projects'));

    expect(await screen.findByTestId('session-expired')).toHaveTextContent('Your session has ended');
    expect(signIn).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', {name: 'Sign in again'}));
    expect(signIn).toHaveBeenCalledTimes(1);
  });

  it('sends the console to sign in when the control plane refuses a call', async () => {
    renderWith(refused('https://cp.test/applications'));

    await waitFor(() => expect(signIn).toHaveBeenCalled());
    expect(screen.getByTestId('session-expired')).toBeInTheDocument();
  });

  it('says nothing for other failures', async () => {
    renderWith(refused('https://cp.test/applications', 500));

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.queryByTestId('session-expired')).not.toBeInTheDocument();
  });
});
