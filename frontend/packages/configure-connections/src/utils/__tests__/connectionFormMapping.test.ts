// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {CONNECTION_FORM_FIELDS, type ConnectionFieldDef} from '../../config/connectionFormFields';
import type {ConnectionResponse} from '../../models/connection';
import {
  MASKED_SECRET,
  emptyFormValues,
  formValuesToRequest,
  outboundAuthenticationFromFormValues,
  responseToFormValues,
  validateConnectionForm,
} from '../connectionFormMapping';

const GOOGLE_FIELDS = CONNECTION_FORM_FIELDS.google;
const OIDC_FIELDS = CONNECTION_FORM_FIELDS.oidc;
const OAUTH_FIELDS = CONNECTION_FORM_FIELDS.oauth;
const TWILIO_FIELDS = CONNECTION_FORM_FIELDS.twilio;
const SMS_GATEWAY_FIELDS = CONNECTION_FORM_FIELDS['sms-gateway'];
const AUTHZEN_PDP_FIELDS = CONNECTION_FORM_FIELDS['authzen-pdp'];
const REDIRECT = 'https://id.acme.io/oauth/callback/google';
const VALID_ACCOUNT_SID = `AC${'a1b2c3d4e5f6'.repeat(2)}01234567`;

describe('emptyFormValues', () => {
  it('blanks every field except the derived redirect URI', () => {
    const values = emptyFormValues(GOOGLE_FIELDS, REDIRECT);
    expect(values['redirectUri']).toBe(REDIRECT);
    expect(values['name']).toBe('');
    expect(values['clientId']).toBe('');
    expect(values['clientSecret']).toBe('');
  });

  it('prefills fields that declare a default value', () => {
    const values = emptyFormValues(SMS_GATEWAY_FIELDS, REDIRECT);
    expect(values['httpMethod']).toBe('POST');
    expect(values['contentType']).toBe('JSON');
    expect(values['url']).toBe('');
    expect(values['apiKeyHeaders']).toBe('');
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
    expect(values['name']).toBe('My Google');
    expect(values['clientId']).toBe('abc');
    expect(values['clientSecret']).toBe('');
    expect(values['scopes']).toBe('openid email profile');
    expect(values['redirectUri']).toBe('https://stored/callback');
  });

  it('falls back to the derived redirect URI when the response has none', () => {
    const response = {id: '1', type: 'google', name: 'X', clientId: 'y'} as ConnectionResponse;
    const values = responseToFormValues(response, GOOGLE_FIELDS, REDIRECT);
    expect(values['redirectUri']).toBe(REDIRECT);
  });

  it('shows masked API-key headers in the SMS gateway form', () => {
    const response = {
      id: '1',
      type: 'sms-gateway',
      name: 'SMS Gateway',
      authentication: {
        type: 'api_key',
        properties: {'Key-1': '******', 'Key-2': '******'},
      },
    } as ConnectionResponse;

    expect(responseToFormValues(response, SMS_GATEWAY_FIELDS, REDIRECT)['apiKeyHeaders']).toBe(
      'Key-1: ******, Key-2: ******',
    );
  });

  it('converts a boolean tokenExchangeEnabled into a "true"/"false" form string', () => {
    const enabled = {
      id: '1',
      type: 'oidc',
      name: 'X',
      clientId: 'y',
      tokenExchangeEnabled: true,
    } as ConnectionResponse;
    expect(responseToFormValues(enabled, OIDC_FIELDS, REDIRECT)['tokenExchangeEnabled']).toBe('true');

    const disabled = {
      id: '1',
      type: 'oidc',
      name: 'X',
      clientId: 'y',
      tokenExchangeEnabled: false,
    } as ConnectionResponse;
    expect(responseToFormValues(disabled, OIDC_FIELDS, REDIRECT)['tokenExchangeEnabled']).toBe('false');
  });

  it('converts numeric AuthZEN PDP response fields into form strings', () => {
    const response = {
      id: '1',
      type: 'authzen-pdp',
      name: 'Cerbos PDP',
      endpoint: 'http://localhost:3592/.well-known/authzen-configuration',
      batchEndpoint: 'http://localhost:3592/access/v1/evaluations',
      timeoutMs: 1000,
      retryCount: 1,
    } as ConnectionResponse;

    const values = responseToFormValues(response, AUTHZEN_PDP_FIELDS, REDIRECT);
    expect(values['timeoutMs']).toBe('1000');
    expect(values['retryCount']).toBe('1');
  });

  it('maps the structured authentication response into flat form state without exposing credentials', () => {
    const response = {
      id: '1',
      type: 'authzen-pdp',
      name: 'Cerbos PDP',
      endpoint: 'https://pdp.example.com/access/v1/evaluation',
      authentication: {
        type: 'basic',
        properties: {username: 'test-user', password: MASKED_SECRET},
      },
    } as ConnectionResponse;

    const values = responseToFormValues(response, AUTHZEN_PDP_FIELDS, REDIRECT);

    expect(values['authenticationScheme']).toBe('BASIC');
    expect(values['basicUsername']).toBe('test-user');
    expect(values['basicPassword']).toBe('');
  });

  it('reads a field from its configured nested response path', () => {
    const fields: ConnectionFieldDef[] = [
      {
        name: 'authMode',
        responsePath: 'authentication.type',
        labelKey: 'test.authMode',
        kind: 'select',
      },
    ];
    const response = {
      id: '1',
      type: 'authzen-pdp',
      authentication: {type: 'bearer'},
    } as ConnectionResponse;

    expect(responseToFormValues(response, fields, REDIRECT)['authMode']).toBe('bearer');
  });
});

describe('outboundAuthenticationFromFormValues', () => {
  it('maps Bearer authentication', () => {
    expect(outboundAuthenticationFromFormValues({authenticationScheme: 'BEARER', bearerToken: ' token '})).toEqual({
      type: 'bearer',
      properties: {token: 'token'},
    });
  });

  it('maps Basic authentication', () => {
    expect(
      outboundAuthenticationFromFormValues({
        authenticationScheme: 'BASIC',
        basicUsername: ' user ',
        basicPassword: ' password ',
      }),
    ).toEqual({type: 'basic', properties: {username: 'user', password: 'password'}});
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

  it('maps missing or NONE authentication to no authentication', () => {
    expect(outboundAuthenticationFromFormValues({})).toEqual({type: 'none'});
    expect(outboundAuthenticationFromFormValues({authenticationScheme: 'NONE'})).toEqual({type: 'none'});
  });
});

describe('formValuesToRequest', () => {
  const base = {name: 'My Google', clientId: 'abc', redirectUri: REDIRECT, scopes: 'openid email', clientSecret: ''};

  it('includes the secret on create and splits scopes into an array', () => {
    const payload = formValuesToRequest({...base, clientSecret: 's3cret'}, GOOGLE_FIELDS, {
      mode: 'create',
    }) as unknown as Record<string, unknown>;
    expect(payload['clientSecret']).toBe('s3cret');
    expect(payload['scopes']).toEqual(['openid', 'email']);
  });

  it('includes trusted token audience when configured', () => {
    const payload = formValuesToRequest(
      {
        ...base,
        authorizationEndpoint: 'https://i/a',
        tokenEndpoint: 'https://i/t',
        tokenExchangeEnabled: 'true',
        trustedTokenAudience: 'my-external-client-id',
      },
      OIDC_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;
    expect(payload['trustedTokenAudience']).toBe('my-external-client-id');
  });

  it('sends the SMS gateway transport fields and omits empty optional headers', () => {
    const payload = formValuesToRequest(
      {name: 'Custom SMS Sender', url: 'https://sms.example.com/send', httpMethod: 'POST', contentType: 'JSON'},
      SMS_GATEWAY_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;

    expect(payload).toEqual({
      name: 'Custom SMS Sender',
      url: 'https://sms.example.com/send',
      httpMethod: 'POST',
      contentType: 'JSON',
    });
  });

  it('sends AuthZEN PDP timing fields as numbers', () => {
    const payload = formValuesToRequest(
      {
        name: 'Cerbos PDP',
        endpoint: 'http://localhost:3592/access/v1/evaluation',
        batchEndpoint: 'http://localhost:3592/access/v1/evaluations',
        timeoutMs: '500',
        retryCount: '1',
      },
      AUTHZEN_PDP_FIELDS,
      {mode: 'edit'},
    ) as unknown as Record<string, unknown>;

    expect(payload).toMatchObject({timeoutMs: 500, retryCount: 1});
    expect(typeof payload['timeoutMs']).toBe('number');
    expect(typeof payload['retryCount']).toBe('number');
  });

  it('converts any number field without relying on its name', () => {
    const fields: ConnectionFieldDef[] = [{name: 'customLimit', labelKey: 'test.customLimit', kind: 'number'}];

    expect(formValuesToRequest({customLimit: '7'}, fields, {mode: 'edit'})).toEqual({customLimit: 7});
  });

  it('converts SMS gateway API-key header rows into structured request headers', () => {
    const payload = formValuesToRequest(
      {
        name: 'Custom SMS Sender',
        url: 'https://sms.example.com/send',
        httpMethod: 'POST',
        contentType: 'JSON',
        apiKeyHeaders: 'X-API-Key: secret, X-Tenant: tenant-1',
      },
      SMS_GATEWAY_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;

    expect(payload['authentication']).toEqual({
      type: 'api_key',
      properties: {'X-API-Key': 'secret', 'X-Tenant': 'tenant-1'},
    });
  });

  it('sends masked SMS gateway API-key headers as retain markers on edit', () => {
    const payload = formValuesToRequest(
      {
        name: 'Custom SMS Sender',
        url: 'https://sms.example.com/send',
        httpMethod: 'POST',
        contentType: 'JSON',
        apiKeyHeaders: 'Key-1: ******, Key-2: ******',
      },
      SMS_GATEWAY_FIELDS,
      {mode: 'edit'},
    ) as unknown as Record<string, unknown>;

    expect(payload['authentication']).toEqual({
      type: 'api_key',
      properties: {'Key-1': '******', 'Key-2': '******'},
    });
  });

  it('sends an empty API-key header list when all stored headers are deleted', () => {
    const payload = formValuesToRequest(
      {
        name: 'Custom SMS Sender',
        url: 'https://sms.example.com/send',
        httpMethod: 'POST',
        contentType: 'JSON',
        apiKeyHeaders: '',
      },
      SMS_GATEWAY_FIELDS,
      {mode: 'edit'},
    ) as unknown as Record<string, unknown>;

    expect(payload['authentication']).toEqual({type: 'none'});
  });

  it('still sends the SMS gateway transport defaults now that neither field is required', () => {
    const values = {
      ...emptyFormValues(SMS_GATEWAY_FIELDS, REDIRECT),
      name: 'Custom SMS Sender',
      url: 'https://sms.example.com/send',
    };

    expect(validateConnectionForm(values, SMS_GATEWAY_FIELDS, 'create')).toEqual({});
    expect(formValuesToRequest(values, SMS_GATEWAY_FIELDS, {mode: 'create'})).toEqual({
      name: 'Custom SMS Sender',
      url: 'https://sms.example.com/send',
      httpMethod: 'POST',
      contentType: 'JSON',
    });
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
    expect(payload['clientSecret']).toBe('new');
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

  it('emits a boolean tokenExchangeEnabled instead of the raw string form value', () => {
    const payload = formValuesToRequest(
      {
        ...base,
        authorizationEndpoint: 'https://i/a',
        tokenEndpoint: 'https://i/t',
        tokenExchangeEnabled: 'true',
      },
      OIDC_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;
    expect(payload['tokenExchangeEnabled']).toBe(true);
    expect(typeof payload['tokenExchangeEnabled']).toBe('boolean');
  });

  it('emits tokenExchangeEnabled as false when the switch is off', () => {
    const payload = formValuesToRequest(
      {
        ...base,
        authorizationEndpoint: 'https://i/a',
        tokenEndpoint: 'https://i/t',
        tokenExchangeEnabled: 'false',
      },
      OIDC_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;
    expect(payload['tokenExchangeEnabled']).toBe(false);
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
        trustedTokenAudience: '',
      },
      OIDC_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;
    expect(payload['authorizationEndpoint']).toBe('https://i/a');
    expect(payload).not.toHaveProperty('userInfoEndpoint');
    expect(payload).not.toHaveProperty('issuer');
    expect(payload).not.toHaveProperty('scopes');
  });
});

describe('validateConnectionForm', () => {
  it('flags required fields on create', () => {
    const errors = validateConnectionForm(emptyFormValues(GOOGLE_FIELDS, REDIRECT), GOOGLE_FIELDS, 'create');
    expect(errors['name']).toBe('connections:validation.required');
    expect(errors['clientId']).toBe('connections:validation.required');
    expect(errors['clientSecret']).toBe('connections:validation.required');
  });

  it('does not require the OAuth 2 user profile endpoint', () => {
    const errors = validateConnectionForm(
      {
        name: 'n',
        clientId: 'c',
        clientSecret: 's',
        redirectUri: REDIRECT,
        authorizationEndpoint: 'https://i/a',
        tokenEndpoint: 'https://i/t',
        userInfoEndpoint: '',
      },
      OAUTH_FIELDS,
      'create',
    );
    expect(errors).not.toHaveProperty('userInfoEndpoint');
  });

  it('does not require the secret on edit', () => {
    const values = {...emptyFormValues(GOOGLE_FIELDS, REDIRECT), name: 'n', clientId: 'c'};
    const errors = validateConnectionForm(values, GOOGLE_FIELDS, 'edit');
    expect(errors).not.toHaveProperty('clientSecret');
  });

  it('rejects an incomplete API key header pair', () => {
    const errors = validateConnectionForm(
      {
        name: 'Custom SMS Sender',
        url: 'https://sms.example.com/send',
        httpMethod: 'POST',
        contentType: 'JSON',
        apiKeyHeaders: 'X-API-Key:',
      },
      SMS_GATEWAY_FIELDS,
      'edit',
    );

    expect(errors['apiKeyHeaders']).toBe('connections:validation.keyValuePair');
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
    expect(bad['authorizationEndpoint']).toBe('connections:validation.url');

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

  it('flags a Twilio account SID that does not match the required format', () => {
    const errors = validateConnectionForm(
      {name: 'n', accountSid: 'not-a-sid', authToken: 't', senderId: '+15005550006'},
      TWILIO_FIELDS,
      'create',
    );
    expect(errors['accountSid']).toBe('connections:validation.accountSid');
  });

  it('accepts a well-formed Twilio account SID', () => {
    const errors = validateConnectionForm(
      {name: 'n', accountSid: VALID_ACCOUNT_SID, authToken: 't', senderId: '+15005550006'},
      TWILIO_FIELDS,
      'create',
    );
    expect(errors).not.toHaveProperty('accountSid');
  });

  it.each(['0', '-1', '1.5', 'abc'])('rejects invalid AuthZEN PDP timeout %s', (timeoutMs) => {
    const errors = validateConnectionForm(
      {...emptyFormValues(AUTHZEN_PDP_FIELDS, REDIRECT), timeoutMs},
      AUTHZEN_PDP_FIELDS,
      'edit',
    );

    expect(errors['timeoutMs']).toBe('connections:validation.positiveInteger');
  });

  it('accepts a positive integer AuthZEN PDP timeout', () => {
    const errors = validateConnectionForm(
      {...emptyFormValues(AUTHZEN_PDP_FIELDS, REDIRECT), timeoutMs: '1'},
      AUTHZEN_PDP_FIELDS,
      'edit',
    );

    expect(errors).not.toHaveProperty('timeoutMs');
  });

  it.each([
    ['BEARER', 'http://pdp.example.com/evaluation', 'endpoint'],
    ['BASIC', 'http://pdp.example.com/evaluation', 'endpoint'],
    ['API_KEY', 'http://pdp.example.com/evaluations', 'batchEndpoint'],
  ])('requires HTTPS for remote %s authenticated PDP endpoints', (authenticationScheme, url, field) => {
    const values = {
      ...emptyFormValues(AUTHZEN_PDP_FIELDS, REDIRECT),
      name: 'PDP',
      endpoint: 'https://pdp.example.com/evaluation',
      batchEndpoint: 'https://pdp.example.com/evaluations',
      authenticationScheme,
      [field]: url,
    };

    expect(validateConnectionForm(values, AUTHZEN_PDP_FIELDS, 'edit')[field]).toBe(
      'connections:validation.authenticatedEndpointHttps',
    );
  });

  it.each([
    ['http://pdp.example.com/evaluation', 'NONE'],
    ['https://pdp.example.com/evaluation', 'BEARER'],
    ['http://localhost:3592/evaluation', 'BASIC'],
    ['http://127.0.0.2:3592/evaluation', 'API_KEY'],
    ['http://[::1]:3592/evaluation', 'BEARER'],
  ])('accepts the PDP endpoint %s with authentication scheme %s', (endpoint, authenticationScheme) => {
    const values = {
      ...emptyFormValues(AUTHZEN_PDP_FIELDS, REDIRECT),
      name: 'PDP',
      endpoint,
      authenticationScheme,
    };

    expect(validateConnectionForm(values, AUTHZEN_PDP_FIELDS, 'edit')).not.toHaveProperty('endpoint');
  });

  it.each(['-1', '1.5', 'abc'])('rejects invalid AuthZEN PDP retry count %s', (retryCount) => {
    const errors = validateConnectionForm(
      {...emptyFormValues(AUTHZEN_PDP_FIELDS, REDIRECT), retryCount},
      AUTHZEN_PDP_FIELDS,
      'edit',
    );

    expect(errors['retryCount']).toBe('connections:validation.nonNegativeInteger');
  });

  it.each(['0', '1'])('accepts AuthZEN PDP retry count %s', (retryCount) => {
    const errors = validateConnectionForm(
      {...emptyFormValues(AUTHZEN_PDP_FIELDS, REDIRECT), retryCount},
      AUTHZEN_PDP_FIELDS,
      'edit',
    );

    expect(errors).not.toHaveProperty('retryCount');
  });

  it('reports the required error before the pattern error for an empty account SID', () => {
    const errors = validateConnectionForm(
      {name: 'n', accountSid: '', authToken: 't', senderId: '+15005550006'},
      TWILIO_FIELDS,
      'create',
    );
    expect(errors['accountSid']).toBe('connections:validation.required');
  });

  it('requires issuer and jwksEndpoint only when tokenExchangeEnabled is on', () => {
    const base = {
      name: 'n',
      clientId: 'c',
      clientSecret: 's',
      redirectUri: REDIRECT,
      authorizationEndpoint: 'https://i/a',
      tokenEndpoint: 'https://i/t',
      issuer: '',
      jwksEndpoint: '',
    };

    const withExchangeOff = validateConnectionForm({...base, tokenExchangeEnabled: 'false'}, OIDC_FIELDS, 'create');
    expect(withExchangeOff).not.toHaveProperty('issuer');
    expect(withExchangeOff).not.toHaveProperty('jwksEndpoint');

    const withExchangeOn = validateConnectionForm({...base, tokenExchangeEnabled: 'true'}, OIDC_FIELDS, 'create');
    expect(withExchangeOn['issuer']).toBe('connections:validation.required');
    expect(withExchangeOn['jwksEndpoint']).toBe('connections:validation.required');
  });

  it('skips validation for a field hidden by revealedBy, even if it would otherwise be invalid', () => {
    const fields: ConnectionFieldDef[] = [
      {name: 'gate', labelKey: 'x', kind: 'switch'},
      {name: 'child', labelKey: 'y', kind: 'url', required: true, revealedBy: 'gate'},
    ];

    const hidden = validateConnectionForm({gate: 'false', child: 'not-a-url'}, fields, 'create');
    expect(hidden).not.toHaveProperty('child');

    const shown = validateConnectionForm({gate: 'true', child: 'not-a-url'}, fields, 'create');
    expect(shown['child']).toBe('connections:validation.url');
  });
});
