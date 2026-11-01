// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

'use client';

import { useManagementRequestConfig } from '@thunderid/nextjs';
import { FormEvent, useState } from 'react';

type ManagementFetcher = NonNullable<ReturnType<typeof useManagementRequestConfig>['fetcher']>;

interface AddIdentityProviderProps {
  callbackUrl: string;
  connectionsUrl: string;
  fetcher: ManagementFetcher;
  onCancel: () => void;
  onCreated: () => void;
}

const FIELDS = [
  { name: 'name', label: 'Name', placeholder: 'Northstar Labs SSO' },
  { name: 'clientId', label: 'Client ID' },
  { name: 'clientSecret', label: 'Client secret', type: 'password' },
  { name: 'authorizationEndpoint', label: 'Authorization endpoint', type: 'url' },
  { name: 'tokenEndpoint', label: 'Token endpoint', type: 'url' },
  { name: 'userInfoEndpoint', label: 'User info endpoint', type: 'url' },
  { name: 'jwksEndpoint', label: 'JWKS endpoint', type: 'url' },
  { name: 'issuer', label: 'Issuer' },
] as const;

/**
 * Adds an OpenID Connect identity provider owned by the admin's company. The ThunderID SDK has no form for
 * this yet, so the app collects the settings and posts them with the SDK's management fetcher to the
 * connections URL it configured, its own /api/admin route, which creates the connection in the company.
 */
export default function AddIdentityProvider({
  callbackUrl,
  connectionsUrl,
  fetcher,
  onCancel,
  onCreated,
}: AddIdentityProviderProps) {
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSaving(true);
    setError(null);
    const values = Object.fromEntries(new FormData(event.currentTarget));
    try {
      const response = await fetcher(`${connectionsUrl}/oidc`, {
        method: 'POST',
        body: JSON.stringify({ ...values, redirectUri: callbackUrl, scopes: ['openid', 'profile', 'email'] }),
      });
      if (response.ok) {
        onCreated();
        return;
      }
      // ThunderID answers { description: { defaultValue } }, and the app's own route answers { error }.
      const body = await response.json().catch(() => undefined);
      setError(body?.description?.defaultValue ?? body?.error ?? 'Could not add the identity provider.');
    } catch {
      setError('Could not add the identity provider.');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <form className="stack" onSubmit={save}>
      <h2>Add identity provider</h2>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      {FIELDS.map((field) => (
        <label key={field.name}>
          {field.label}
          <input
            name={field.name}
            type={'type' in field ? field.type : 'text'}
            placeholder={'placeholder' in field ? field.placeholder : undefined}
            required
          />
        </label>
      ))}
      <div className="form-actions">
        <button type="button" className="button ghost" onClick={onCancel} disabled={isSaving}>
          Cancel
        </button>
        <button type="submit" className="button primary" disabled={isSaving}>
          {isSaving ? 'Adding…' : 'Add identity provider'}
        </button>
      </div>
    </form>
  );
}
