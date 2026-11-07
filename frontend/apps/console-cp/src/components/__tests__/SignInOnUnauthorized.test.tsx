// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider, useQuery} from '@tanstack/react-query';
import {render, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SignInOnUnauthorized from '../SignInOnUnauthorized';

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
  useQuery({
    queryKey: ['failing', (error as {config?: {url?: string}}).config?.url],
    queryFn: () => Promise.reject(error),
    retry: false,
  });
  return null;
}

function renderWith(error: Error): void {
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <SignInOnUnauthorized />
      <Failing error={error} />
    </QueryClientProvider>,
  );
}

describe('SignInOnUnauthorized', () => {
  beforeEach(() => signIn.mockClear());

  it('sends the console to sign in when the control plane refuses a call', async () => {
    renderWith(refused('https://cp.test/applications'));

    await waitFor(() => expect(signIn).toHaveBeenCalled());
  });

  it('keeps the page when signing in again cannot start, and starts it again on the next refusal', async () => {
    signIn.mockRejectedValueOnce(new Error('sign-in service unreachable'));
    renderWith(refused('https://cp.test/applications'));
    await waitFor(() => expect(signIn).toHaveBeenCalledTimes(1));

    renderWith(refused('https://cp.test/agents'));
    await waitFor(() => expect(signIn).toHaveBeenCalledTimes(2));
  });

  it('starts one sign-in when several control plane calls are refused together', async () => {
    let finish: () => void = () => undefined;
    signIn.mockImplementationOnce(() => new Promise<void>((resolve) => (finish = resolve)));
    render(
      <QueryClientProvider client={new QueryClient()}>
        <SignInOnUnauthorized />
        <Failing error={refused('https://cp.test/applications')} />
        <Failing error={refused('https://cp.test/agents')} />
      </QueryClientProvider>,
    );

    await waitFor(() => expect(signIn).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(signIn).toHaveBeenCalledTimes(1);
    finish();
  });

  it('takes no other host that starts with the same text for the control plane', async () => {
    renderWith(refused('https://cp.test.other.example/projects'));

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(signIn).not.toHaveBeenCalled();
  });

  it("leaves another service's refusal to the page that read it", async () => {
    renderWith(refused('https://org-mgt.test/projects'));

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(signIn).not.toHaveBeenCalled();
  });

  it('does nothing for other failures', async () => {
    renderWith(refused('https://cp.test/applications', 500));

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(signIn).not.toHaveBeenCalled();
  });
});
