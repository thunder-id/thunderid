// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

'use client';

import { FlowVariablesForm, useManagementRequestConfig } from '@thunderid/nextjs';
import { useEffect, useState } from 'react';
import AddIdentityProvider from './AddIdentityProvider';

interface EnterpriseSsoProps {
  callbackUrl: string;
  ouHandle?: string;
  ouId: string;
  ouName?: string;
  signInFlowId: string;
}

interface IdentityProvider {
  id: string;
  name: string;
  type: string;
}

/**
 * The company's enterprise SSO. `FlowVariablesForm` from the ThunderID SDK sets the company's value of the
 * shared sign-in flow's `enterpriseIdp` variable, picking from the connections the company owns or was
 * shared. The side panel lists those identity providers and adds an OpenID Connect one owned by the
 * company. Both use the SDK's management request config: the connections URL the app configured (its own
 * /api/admin route) and the SDK's fetcher, which sends the request from the server with the user's session.
 */
export default function EnterpriseSso({ callbackUrl, ouHandle, ouId, ouName, signInFlowId }: EnterpriseSsoProps) {
  const { baseUrl, fetcher: sdkFetcher, url } = useManagementRequestConfig('connections');
  const connectionsUrl = url ?? `${baseUrl}/connections`;
  // The SDK always supplies its fetcher; the browser's fetch also reaches the app's own route by its cookie.
  const fetcher = sdkFetcher ?? fetch;
  const [isAdding, setIsAdding] = useState(false);
  // Changes after an identity provider is added, so the list and the form load the connections again.
  const [connectionsVersion, setConnectionsVersion] = useState(0);
  const [providers, setProviders] = useState<IdentityProvider[] | null>(null);
  const [listError, setListError] = useState<string | null>(null);

  useEffect(() => {
    let isCurrent = true;
    const query = new URLSearchParams({ category: 'identity-provider', filter: `ouId eq "${ouId}"`, limit: '100' });
    fetcher(`${connectionsUrl}?${query}`, { method: 'GET' })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(response.statusText);
        }
        const page: { connections?: IdentityProvider[] } = await response.json();
        if (isCurrent) {
          setListError(null);
          setProviders(page.connections ?? []);
        }
      })
      .catch(() => isCurrent && setListError('Could not load your identity providers.'));
    return () => {
      isCurrent = false;
    };
  }, [connectionsUrl, fetcher, ouId, connectionsVersion]);

  return (
    <div className="stack">
      <section className="panel steps">
        <h2>How it works</h2>
        <ol>
          <li>
            In your identity provider, register an OpenID Connect application with the callback URL{' '}
            <code>{callbackUrl}</code>.
          </li>
          <li>Add the identity provider here with that application&apos;s client ID, secret and endpoints.</li>
          <li>Select it as your company&apos;s enterprise SSO connection and save.</li>
          <li>
            Employees choose <strong>Sign in with enterprise SSO</strong> and enter{' '}
            <code>{ouHandle ?? 'your organization handle'}</code>. First-time employees join your company as members.
          </li>
        </ol>
      </section>

      <div className="two-column">
        <section className="panel">
          <FlowVariablesForm
            key={connectionsVersion}
            flowId={signInFlowId}
            ouId={ouId}
            ouName={ouName}
            showScopeNotice={Boolean(ouName)}
            title="Enterprise SSO connection"
            onCreateConnection={() => setIsAdding(true)}
          />
        </section>

        <section className="panel">
          {isAdding ? (
            <AddIdentityProvider
              callbackUrl={callbackUrl}
              connectionsUrl={connectionsUrl}
              fetcher={fetcher}
              onCancel={() => setIsAdding(false)}
              onCreated={() => {
                setIsAdding(false);
                setConnectionsVersion((version) => version + 1);
              }}
            />
          ) : (
            <>
              <div className="panel-heading">
                <h2>Identity providers</h2>
                <button type="button" className="button primary small" onClick={() => setIsAdding(true)}>
                  Add identity provider
                </button>
              </div>
              {listError && (
                <p className="alert error" role="alert">
                  {listError}
                </p>
              )}
              {!listError && providers === null && <p className="muted">Loading identity providers…</p>}
              {providers?.length === 0 && (
                <p className="muted">
                  Your company has no identity provider yet. Add your OpenID Connect identity provider, then select it
                  as the enterprise SSO connection.
                </p>
              )}
              {providers && providers.length > 0 && (
                <table className="table">
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Type</th>
                    </tr>
                  </thead>
                  <tbody>
                    {providers.map((provider) => (
                      <tr key={provider.id}>
                        <td>{provider.name}</td>
                        <td>{provider.type.toUpperCase()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </>
          )}
        </section>
      </div>
    </div>
  );
}
