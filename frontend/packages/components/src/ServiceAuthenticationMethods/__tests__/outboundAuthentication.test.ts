// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {
  getAPIKeyHeaderProblems,
  getServiceAuthenticationState,
  MASKED_SECRET,
  outboundAuthenticationFromFormValues,
  outboundAuthenticationToFormValues,
} from '../outboundAuthentication';

describe('outboundAuthenticationFromFormValues', () => {
  it('maps Bearer authentication, sending the token as entered', () => {
    expect(outboundAuthenticationFromFormValues({authenticationScheme: 'BEARER', bearerToken: ' token '})).toEqual({
      type: 'bearer',
      properties: {token: ' token '},
    });
  });

  it('maps Basic authentication, trimming only the username', () => {
    expect(
      outboundAuthenticationFromFormValues({
        authenticationScheme: 'BASIC',
        basicUsername: ' user ',
        basicPassword: ' password ',
      }),
    ).toEqual({type: 'basic', properties: {username: 'user', password: ' password '}});
  });

  it('maps API-key headers as structured values', () => {
    expect(
      outboundAuthenticationFromFormValues({
        authenticationScheme: 'API_KEY',
        httpHeaders: 'X-API-Key: secret, X-Tenant: acme',
      }),
    ).toEqual({
      type: 'api_key',
      properties: {'X-API-Key': 'secret', 'X-Tenant': 'acme'},
    });
  });

  it('never turns API key authentication without headers into no authentication', () => {
    expect(outboundAuthenticationFromFormValues({authenticationScheme: 'API_KEY', httpHeaders: ''})).toEqual({
      type: 'api_key',
      properties: {},
    });
  });

  it('maps missing or NONE authentication to no authentication', () => {
    expect(outboundAuthenticationFromFormValues({})).toEqual({type: 'none'});
    expect(outboundAuthenticationFromFormValues({authenticationScheme: 'NONE'})).toEqual({type: 'none'});
  });
});

describe('outboundAuthenticationToFormValues', () => {
  it('maps no authentication to the NONE method', () => {
    expect(outboundAuthenticationToFormValues(undefined)).toEqual({
      authenticationScheme: 'NONE',
      bearerToken: '',
      basicUsername: '',
      basicPassword: '',
      httpHeaders: '',
    });
  });

  it('blanks a masked bearer token', () => {
    expect(outboundAuthenticationToFormValues({type: 'bearer', properties: {token: MASKED_SECRET}})).toMatchObject({
      authenticationScheme: 'BEARER',
      bearerToken: '',
    });
  });

  it('keeps the Basic username and blanks the masked password', () => {
    expect(
      outboundAuthenticationToFormValues({type: 'basic', properties: {username: 'svc', password: MASKED_SECRET}}),
    ).toMatchObject({authenticationScheme: 'BASIC', basicUsername: 'svc', basicPassword: ''});
  });

  it('serializes API key headers with their masked values', () => {
    expect(
      outboundAuthenticationToFormValues({
        type: 'api_key',
        properties: {'X-API-Key': MASKED_SECRET, 'X-Tenant': 'acme'},
      }),
    ).toMatchObject({authenticationScheme: 'API_KEY', httpHeaders: 'X-API-Key: ******, X-Tenant: acme'});
  });
});

describe('getAPIKeyHeaderProblems', () => {
  it('accepts headers the server accepts', () => {
    expect(getAPIKeyHeaderProblems('X-API-Key: secret, Authorization: Token abc, x_tenant~1: acme', '')).toEqual([
      undefined,
      undefined,
      undefined,
    ]);
  });

  it('refuses a header name that is not an HTTP token', () => {
    expect(getAPIKeyHeaderProblems('X API Key: secret, Clé: v, X-Key/1: v', '')).toEqual([
      'invalidName',
      'invalidName',
      'invalidName',
    ]);
  });

  it('refuses a reserved header in any case', () => {
    expect(getAPIKeyHeaderProblems('host: a, Content-Type: b, PROXY-AUTHORIZATION: c, Keep-Alive: d', '')).toEqual([
      'reservedName',
      'reservedName',
      'reservedName',
      'reservedName',
    ]);
  });

  it('refuses a header added twice, whatever its case', () => {
    expect(getAPIKeyHeaderProblems('X-API-Key: a, x-api-key: b', '')).toEqual([undefined, 'duplicateName']);
  });

  it('needs a name and a value', () => {
    expect(getAPIKeyHeaderProblems(': secret, X-API-Key:', '')).toEqual(['nameRequired', 'valueRequired']);
  });

  it('keeps a masked value only for a stored header that can be kept', () => {
    const stored = 'X-API-Key: ******';
    expect(getAPIKeyHeaderProblems('X-API-Key: ******, X-New: ******', stored)).toEqual([undefined, 'valueReentry']);
    expect(getAPIKeyHeaderProblems('X-API-Key: ******', '')).toEqual(['valueReentry']);
  });
});

describe('getServiceAuthenticationState', () => {
  const options = {replacing: false, targetChanged: false, requiredMessage: 'Required'};
  const storedBearer = {type: 'bearer', properties: {token: MASKED_SECRET}};
  const storedHeaders = {type: 'api_key', properties: {'X-API-Key': MASKED_SECRET}};

  it('keeps a stored bearer token without requiring it again', () => {
    const state = getServiceAuthenticationState(
      outboundAuthenticationToFormValues(storedBearer),
      storedBearer,
      options,
    );
    expect(state.hasStoredCredentialForSelectedScheme).toBe(true);
    expect(state.hasRequiredAuthenticationCredential).toBe(true);
    expect(state.authenticationChanged).toBe(false);
  });

  it('requires the bearer token again when replacing it', () => {
    const state = getServiceAuthenticationState(outboundAuthenticationToFormValues(storedBearer), storedBearer, {
      ...options,
      replacing: true,
    });
    expect(state.authenticationErrors.bearerToken).toBe('Required');
    expect(state.authenticationChanged).toBe(true);
  });

  it('requires the credential again when the target changes', () => {
    const state = getServiceAuthenticationState(outboundAuthenticationToFormValues(storedBearer), storedBearer, {
      ...options,
      targetChanged: true,
    });
    expect(state.requiresFreshCredentials).toBe(true);
    expect(state.hasStoredCredentialForSelectedScheme).toBe(false);
    expect(state.authenticationErrors.bearerToken).toBe('Required');
  });

  it('treats an entered credential as a change', () => {
    const state = getServiceAuthenticationState(
      {authenticationScheme: 'BEARER', bearerToken: 'new'},
      storedBearer,
      options,
    );
    expect(state.hasEnteredAuthenticationCredential).toBe(true);
    expect(state.authenticationChanged).toBe(true);
  });

  it('treats a different method as a change and requires its credential', () => {
    const state = getServiceAuthenticationState({authenticationScheme: 'BASIC'}, storedBearer, options);
    expect(state.authenticationChanged).toBe(true);
    expect(state.authenticationErrors).toEqual({basicUsername: 'Required', basicPassword: 'Required'});
    expect(state.hasRequiredAuthenticationCredential).toBe(false);
  });

  it('keeps stored API key headers until they change', () => {
    const values = outboundAuthenticationToFormValues(storedHeaders);
    const kept = getServiceAuthenticationState(values, storedHeaders, options);
    expect(kept.hasStoredHTTPHeaders).toBe(true);
    expect(kept.hasRequiredAuthenticationCredential).toBe(true);
    expect(kept.authenticationChanged).toBe(false);

    const changed = getServiceAuthenticationState({...values, httpHeaders: 'X-API-Key: new'}, storedHeaders, options);
    expect(changed.authenticationChanged).toBe(true);
  });

  it('rejects masked API key headers when the target changes', () => {
    const state = getServiceAuthenticationState(outboundAuthenticationToFormValues(storedHeaders), storedHeaders, {
      ...options,
      targetChanged: true,
    });
    expect(state.requiresFreshCredentials).toBe(true);
    expect(state.hasStoredHTTPHeaders).toBe(false);
    expect(state.authenticationErrors.httpHeaders).toBe('Required');
  });

  it('exposes the stored API key headers, which keep the stored credential when restored', () => {
    const state = getServiceAuthenticationState(
      {authenticationScheme: 'API_KEY', httpHeaders: 'X-API-Key: ******'},
      storedHeaders,
      options,
    );
    expect(state.storedHTTPHeaders).toBe('X-API-Key: ******');
    expect(state.hasRequiredAuthenticationCredential).toBe(true);
    expect(state.authenticationChanged).toBe(false);
  });

  it('requires a header once the stored API key headers are removed', () => {
    const state = getServiceAuthenticationState(
      {authenticationScheme: 'API_KEY', httpHeaders: ''},
      storedHeaders,
      options,
    );
    expect(state.authenticationErrors.httpHeaders).toBe('Required');
    expect(state.hasRequiredAuthenticationCredential).toBe(false);
    expect(state.authenticationChanged).toBe(true);
  });

  it('keeps a stored header beside a new one', () => {
    const state = getServiceAuthenticationState(
      {authenticationScheme: 'API_KEY', httpHeaders: 'X-API-Key: ******, X-Tenant: acme'},
      storedHeaders,
      options,
    );
    expect(state.hasRequiredAuthenticationCredential).toBe(true);
    expect(state.authenticationChanged).toBe(true);
  });

  it('refuses an API key header the server refuses', () => {
    const state = getServiceAuthenticationState(
      {authenticationScheme: 'API_KEY', httpHeaders: 'Host: example.com'},
      undefined,
      options,
    );
    expect(state.authenticationErrors.httpHeaders).toBe('Required');
  });

  it('needs nothing for the NONE method', () => {
    const state = getServiceAuthenticationState({authenticationScheme: 'NONE'}, undefined, {
      ...options,
      targetChanged: true,
    });
    expect(state.hasRequiredAuthenticationCredential).toBe(true);
    expect(state.requiresFreshCredentials).toBe(false);
    expect(state.authenticationChanged).toBe(false);
  });
});
