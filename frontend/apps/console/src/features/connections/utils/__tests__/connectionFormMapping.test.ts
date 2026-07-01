/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {describe, expect, it} from 'vitest';
import {CONNECTION_FORM_FIELDS} from '../../config/connectionFormFields';
import type {ConnectionResponse} from '../../models/connection';
import {
  emptyFormValues,
  formValuesToRequest,
  responseToFormValues,
  validateConnectionForm,
} from '../connectionFormMapping';

const GOOGLE_FIELDS = CONNECTION_FORM_FIELDS.google;
const OIDC_FIELDS = CONNECTION_FORM_FIELDS.oidc;
const REDIRECT = 'https://id.acme.io/oauth/callback/google';

describe('emptyFormValues', () => {
  it('blanks every field except the derived redirect URI', () => {
    const values = emptyFormValues(GOOGLE_FIELDS, REDIRECT);
    expect(values.redirectUri).toBe(REDIRECT);
    expect(values.name).toBe('');
    expect(values.clientId).toBe('');
    expect(values.clientSecret).toBe('');
  });
});

describe('responseToFormValues', () => {
  it('joins scopes, blanks the secret, and copies plain fields', () => {
    const response = {
      id: '1',
      type: 'google',
      name: 'My Google',
      clientId: 'abc',
      clientSecret: '******',
      redirectUri: 'https://stored/callback',
      scopes: ['openid', 'email', 'profile'],
    } as ConnectionResponse;

    const values = responseToFormValues(response, GOOGLE_FIELDS, REDIRECT);
    expect(values.name).toBe('My Google');
    expect(values.clientId).toBe('abc');
    expect(values.clientSecret).toBe('');
    expect(values.scopes).toBe('openid email profile');
    expect(values.redirectUri).toBe('https://stored/callback');
  });

  it('falls back to the derived redirect URI when the response has none', () => {
    const response = {id: '1', type: 'google', name: 'X', clientId: 'y'} as ConnectionResponse;
    const values = responseToFormValues(response, GOOGLE_FIELDS, REDIRECT);
    expect(values.redirectUri).toBe(REDIRECT);
  });
});

describe('formValuesToRequest', () => {
  const base = {name: 'My Google', clientId: 'abc', redirectUri: REDIRECT, scopes: 'openid email', clientSecret: ''};

  it('includes the secret on create and splits scopes into an array', () => {
    const payload = formValuesToRequest({...base, clientSecret: 's3cret'}, GOOGLE_FIELDS, {
      mode: 'create',
    }) as unknown as Record<string, unknown>;
    expect(payload.clientSecret).toBe('s3cret');
    expect(payload.scopes).toEqual(['openid', 'email']);
  });

  it('omits the secret on edit when not replacing (keep stored value)', () => {
    const payload = formValuesToRequest(base, GOOGLE_FIELDS, {
      mode: 'edit',
      secretReplaced: false,
    }) as unknown as Record<string, unknown>;
    expect(payload).not.toHaveProperty('clientSecret');
  });

  it('includes the secret on edit when replacing with a value', () => {
    const payload = formValuesToRequest({...base, clientSecret: 'new'}, GOOGLE_FIELDS, {
      mode: 'edit',
      secretReplaced: true,
    }) as unknown as Record<string, unknown>;
    expect(payload.clientSecret).toBe('new');
  });

  it('omits the secret on edit when replacing but left empty', () => {
    const payload = formValuesToRequest({...base, clientSecret: ''}, GOOGLE_FIELDS, {
      mode: 'edit',
      secretReplaced: true,
    }) as unknown as Record<string, unknown>;
    expect(payload).not.toHaveProperty('clientSecret');
  });

  it('never sends the masked placeholder back', () => {
    const payload = formValuesToRequest({...base, clientSecret: '******'}, GOOGLE_FIELDS, {
      mode: 'edit',
      secretReplaced: true,
    }) as unknown as Record<string, unknown>;
    expect(payload).not.toHaveProperty('clientSecret');
  });

  it('omits empty optional fields but keeps required ones', () => {
    const payload = formValuesToRequest(
      {
        name: 'n',
        clientId: 'c',
        clientSecret: 's',
        redirectUri: REDIRECT,
        authorizationEndpoint: 'https://i/a',
        tokenEndpoint: 'https://i/t',
        issuer: '',
        userInfoEndpoint: '',
        jwksEndpoint: '',
        scopes: '',
      },
      OIDC_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;
    expect(payload.authorizationEndpoint).toBe('https://i/a');
    expect(payload).not.toHaveProperty('userInfoEndpoint');
    expect(payload).not.toHaveProperty('issuer');
    expect(payload).not.toHaveProperty('scopes');
  });
});

describe('validateConnectionForm', () => {
  it('flags required fields on create', () => {
    const errors = validateConnectionForm(emptyFormValues(GOOGLE_FIELDS, REDIRECT), GOOGLE_FIELDS, 'create');
    expect(errors.name).toBe('connections:validation.required');
    expect(errors.clientId).toBe('connections:validation.required');
    expect(errors.clientSecret).toBe('connections:validation.required');
  });

  it('does not require the secret on edit', () => {
    const values = {...emptyFormValues(GOOGLE_FIELDS, REDIRECT), name: 'n', clientId: 'c'};
    const errors = validateConnectionForm(values, GOOGLE_FIELDS, 'edit');
    expect(errors).not.toHaveProperty('clientSecret');
  });

  it('flags invalid URLs and accepts valid ones', () => {
    const bad = validateConnectionForm(
      {
        name: 'n',
        clientId: 'c',
        clientSecret: 's',
        redirectUri: REDIRECT,
        authorizationEndpoint: 'not-a-url',
        tokenEndpoint: 'https://i/t',
      },
      OIDC_FIELDS,
      'create',
    );
    expect(bad.authorizationEndpoint).toBe('connections:validation.url');

    const good = validateConnectionForm(
      {
        name: 'n',
        clientId: 'c',
        clientSecret: 's',
        redirectUri: REDIRECT,
        authorizationEndpoint: 'https://i/a',
        tokenEndpoint: 'https://i/t',
      },
      OIDC_FIELDS,
      'create',
    );
    expect(good).not.toHaveProperty('authorizationEndpoint');
  });
});
