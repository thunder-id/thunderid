// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {CONNECTION_FORM_FIELDS, type ConnectionFieldDef} from '../../config/connectionFormFields';
import {AUTHENTICATION_METHOD_FIELD_NAMES} from '../../models/authentication-methods';
import type {ConnectionResponse, OutboundAuthMethod} from '../../models/connection';
import {
  AUTH_ADDITIONAL_PROPERTIES_FIELD,
  AUTH_TYPE_FIELD,
  authenticationAdditionalPropertiesError,
  authPropertyField,
  emptyFormValues,
  formValuesToRequest,
  MASKED_SECRET,
  outboundAuthenticationFromFormValues,
  HTTPS_REQUIRED_WITH_AUTHENTICATION_ERROR,
  responseToFormValues,
  urlOriginChanged,
  validateConnectionForm,
} from '../connectionFormMapping';
import {encodeKeyValuePairs} from '../keyValuePairs';

const GOOGLE_FIELDS = CONNECTION_FORM_FIELDS.google;
const OIDC_FIELDS = CONNECTION_FORM_FIELDS.oidc;
const OAUTH_FIELDS = CONNECTION_FORM_FIELDS.oauth;
const TWILIO_FIELDS = CONNECTION_FORM_FIELDS.twilio;
const SMS_GATEWAY_FIELDS = CONNECTION_FORM_FIELDS['sms-gateway'];
const SMS_GENERAL_FIELDS = SMS_GATEWAY_FIELDS.filter((field) => !AUTHENTICATION_METHOD_FIELD_NAMES.has(field.name));
const AUTHZEN_PDP_FIELDS = CONNECTION_FORM_FIELDS['authzen-pdp'];
const SMTP_FIELDS = CONNECTION_FORM_FIELDS['email-smtp'];
const HTTP_FIELDS = CONNECTION_FORM_FIELDS.http;
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
    expect(values['httpHeaders']).toBe('');
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

  it('falls back to the derived redirect URI when the response value is empty', () => {
    const response = {id: '1', type: 'google', name: 'X', clientId: 'y', redirectUri: ''} as ConnectionResponse;
    expect(responseToFormValues(response, GOOGLE_FIELDS, REDIRECT)['redirectUri']).toBe(REDIRECT);
  });

  it('shows masked API-key headers in the SMS gateway authentication form', () => {
    const response = {
      id: '1',
      type: 'sms-gateway',
      ouId: 'ou-1',
      name: 'SMS Gateway',
      authentication: {
        type: 'api_key',
        properties: {'Key-1': '******', 'Key-2': '******'},
      },
    } as ConnectionResponse;

    const values = responseToFormValues(response, SMS_GATEWAY_FIELDS, REDIRECT);
    expect(values['httpHeaders']).toBe('Key-1: ******, Key-2: ******');
    expect(values['authenticationScheme']).toBe('API_KEY');
  });

  it('keeps SMS gateway general headers separate from authentication', () => {
    const response = {
      id: '1',
      type: 'sms-gateway',
      ouId: 'ou-1',
      name: 'SMS Gateway',
      httpHeaders: 'X-Campaign: campaign-1',
      authentication: {type: 'api_key', properties: {'X-Api-Key': '******'}},
    } as ConnectionResponse;
    const values = responseToFormValues(response, SMS_GATEWAY_FIELDS, REDIRECT);
    expect(values['smsHttpHeaders']).toBe('X-Campaign: campaign-1');
    expect(values['httpHeaders']).toBe('X-Api-Key: ******');

    const request = formValuesToRequest(values, SMS_GENERAL_FIELDS, {
      mode: 'edit',
      excludeGenericAuthentication: true,
    });
    expect(request).toMatchObject({
      httpHeaders: 'X-Campaign: campaign-1',
    });
    expect(request).not.toHaveProperty('authentication');
    expect(outboundAuthenticationFromFormValues(values)).toEqual({
      type: 'api_key',
      properties: {'X-Api-Key': '******'},
    });
  });

  it('sends SMS gateway general headers in the original string format', () => {
    const values = emptyFormValues(SMS_GATEWAY_FIELDS, REDIRECT);
    values['name'] = 'SMS Gateway';
    values['url'] = 'https://sms.example.com/send';
    values['smsHttpHeaders'] = 'X-Campaign: campaign-1';
    const request = formValuesToRequest(values, SMS_GATEWAY_FIELDS, {mode: 'create'});
    expect(request).toMatchObject({httpHeaders: 'X-Campaign: campaign-1'});
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
      ouId: 'ou-1',
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
      ouId: 'ou-1',
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

  // The sender name is optional, and clearing it must drop it from the payload rather than send
  // an empty display name the server would store.
  it('sends the SMTP sender name only when it has a value', () => {
    const values = {
      ...emptyFormValues(SMTP_FIELDS, REDIRECT),
      fromAddress: 'noreply@acme.io',
      host: 'smtp.acme.io',
      name: 'Corp SMTP',
    };

    expect(formValuesToRequest(values, SMTP_FIELDS, {mode: 'create'})).toEqual({
      name: 'Corp SMTP',
      host: 'smtp.acme.io',
      port: 587,
      fromAddress: 'noreply@acme.io',
      tls: 'starttls',
    });

    const named = formValuesToRequest({...values, fromName: 'Acme Support'}, SMTP_FIELDS, {
      mode: 'create',
    }) as unknown as Record<string, unknown>;
    expect(named['fromName']).toBe('Acme Support');
  });

  it('maps SMS gateway authentication separately from general headers', () => {
    const values = {
      ...emptyFormValues(SMS_GATEWAY_FIELDS, REDIRECT),
      name: 'Custom SMS Sender',
      url: 'https://sms.example.com/send',
      smsHttpHeaders: 'X-Campaign: campaign-1',
      authenticationScheme: 'API_KEY',
      httpHeaders: 'X-API-Key: secret',
    };
    const general = formValuesToRequest(values, SMS_GENERAL_FIELDS, {
      mode: 'edit',
      excludeGenericAuthentication: true,
    });
    expect(general).toMatchObject({httpHeaders: 'X-Campaign: campaign-1'});
    expect(general).not.toHaveProperty('authentication');
    expect(outboundAuthenticationFromFormValues(values)).toEqual({
      type: 'api_key',
      properties: {'X-API-Key': 'secret'},
    });
    expect(outboundAuthenticationFromFormValues({...values, authenticationScheme: 'NONE'})).toEqual({type: 'none'});
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
        httpHeaders: 'X-API-Key:',
      },
      SMS_GATEWAY_FIELDS,
      'edit',
    );

    expect(errors['httpHeaders']).toBe('connections:validation.keyValuePair');
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

  it.each(['BEARER', 'BASIC', 'API_KEY'])(
    'requires HTTPS for a remote SMS gateway with %s authentication',
    (authenticationScheme) => {
      const values = {
        ...emptyFormValues(SMS_GATEWAY_FIELDS, REDIRECT),
        url: 'http://sms.example.com/send',
        authenticationScheme,
      };
      expect(validateConnectionForm(values, SMS_GATEWAY_FIELDS, 'edit')['url']).toBe(
        'connections:validation.authenticatedEndpointHttps',
      );
      values.url = 'http://localhost:8080/send';
      expect(validateConnectionForm(values, SMS_GATEWAY_FIELDS, 'edit')).not.toHaveProperty('url');
    },
  );

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

  it('rejects an SMTP from address that is not an email address', () => {
    const values = {
      ...emptyFormValues(SMTP_FIELDS, REDIRECT),
      fromAddress: 'noreply',
      host: 'smtp.acme.io',
      name: 'Corp SMTP',
    };

    expect(validateConnectionForm(values, SMTP_FIELDS, 'create')['fromAddress']).toBe(
      'connections:validation.emailAddress',
    );
    expect(validateConnectionForm({...values, fromAddress: 'noreply@acme.io'}, SMTP_FIELDS, 'create')).toEqual({});
  });

  // The backend rejects this combination with a message the console is not allowed to surface,
  // so the form has to catch it before the request is sent.
  it('rejects credentials over an unencrypted transport', () => {
    const values = {
      ...emptyFormValues(SMTP_FIELDS, REDIRECT),
      [AUTH_TYPE_FIELD]: 'basic',
      fromAddress: 'noreply@acme.io',
      host: 'smtp.acme.io',
      name: 'Corp SMTP',
      tls: 'none',
    };

    expect(validateConnectionForm(values, SMTP_FIELDS, 'create')['tls']).toBe(
      'connections:validation.tlsRequiredForAuthentication',
    );
    expect(validateConnectionForm({...values, tls: 'starttls'}, SMTP_FIELDS, 'create')).toEqual({});
    expect(validateConnectionForm({...values, [AUTH_TYPE_FIELD]: 'none'}, SMTP_FIELDS, 'create')).toEqual({});
  });

  // The rule is keyed off the transport security field, so a vendor without one is unaffected.
  it('leaves a vendor that does not secure its own transport alone', () => {
    const values = {
      ...emptyFormValues(SMS_GATEWAY_FIELDS, REDIRECT),
      [AUTH_TYPE_FIELD]: 'basic',
      name: 'Custom SMS Sender',
      url: 'https://sms.example.com/send',
    };

    expect(validateConnectionForm(values, SMS_GATEWAY_FIELDS, 'create')).toEqual({});
  });
});

describe('outbound authentication mapping', () => {
  const BASE_SMTP = {
    fromAddress: 'noreply@acme.io',
    host: 'smtp.acme.io',
    name: 'Corp SMTP',
    port: '587',
    tls: 'starttls',
  };

  it('nests the namespaced form values under authentication', () => {
    const request = formValuesToRequest(
      {
        ...BASE_SMTP,
        [AUTH_TYPE_FIELD]: 'basic',
        [authPropertyField('password')]: 's3cret',
        [authPropertyField('username')]: 'mailer',
      },
      SMTP_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;

    expect(request['authentication']).toEqual({
      properties: {password: 's3cret', username: 'mailer'},
      type: 'basic',
    });
    // The flat keys must not leak into the payload alongside the nested object.
    expect(request['authentication.type']).toBeUndefined();
  });

  it('sends the method alone when it takes no fields', () => {
    const request = formValuesToRequest({...BASE_SMTP, [AUTH_TYPE_FIELD]: 'none'}, SMTP_FIELDS, {
      mode: 'create',
    }) as unknown as Record<string, unknown>;

    expect(request['authentication']).toEqual({type: 'none'});
  });

  it('omits authentication entirely when the form carries none', () => {
    const request = formValuesToRequest(BASE_SMTP, SMTP_FIELDS, {mode: 'create'}) as unknown as Record<string, unknown>;

    expect(request['authentication']).toBeUndefined();
  });

  // A blank credential is how the form says "keep the stored one", so it must not be sent.
  it('omits a blank credential but keeps the other fields', () => {
    const request = formValuesToRequest(
      {
        ...BASE_SMTP,
        [AUTH_TYPE_FIELD]: 'basic',
        [authPropertyField('password')]: '',
        [authPropertyField('username')]: 'mailer',
      },
      SMTP_FIELDS,
      {mode: 'edit'},
    ) as unknown as Record<string, unknown>;

    expect(request['authentication']).toEqual({properties: {username: 'mailer'}, type: 'basic'});
  });

  // Sending the mask back would store the literal asterisks as the password.
  it('never sends the masked placeholder back', () => {
    const request = formValuesToRequest(
      {
        ...BASE_SMTP,
        [AUTH_TYPE_FIELD]: 'basic',
        [authPropertyField('password')]: MASKED_SECRET,
        [authPropertyField('username')]: 'mailer',
      },
      SMTP_FIELDS,
      {mode: 'edit'},
    ) as unknown as Record<string, unknown>;

    expect(request['authentication']).toEqual({properties: {username: 'mailer'}, type: 'basic'});
  });

  it('flattens a response into namespaced values and blanks the masked credential', () => {
    const response = {
      authentication: {
        properties: {password: MASKED_SECRET, username: 'mailer'},
        type: 'basic',
      },
      fromAddress: 'noreply@acme.io',
      host: 'smtp.acme.io',
      id: '1',
      name: 'Corp SMTP',
      port: 587,
      tls: 'starttls',
      type: 'email-smtp',
    } as unknown as ConnectionResponse;

    const values = responseToFormValues(response, SMTP_FIELDS, REDIRECT);

    expect(values[AUTH_TYPE_FIELD]).toBe('basic');
    expect(values[authPropertyField('username')]).toBe('mailer');
    expect(values[authPropertyField('password')]).toBe('');
  });

  // A connection stored before outbound authentication existed has no block at all; the form
  // must not invent a selection for it.
  it('leaves the method unset when the response carries no authentication', () => {
    const response = {
      fromAddress: 'noreply@acme.io',
      host: 'smtp.acme.io',
      id: '1',
      name: 'Corp SMTP',
      port: 587,
      type: 'email-smtp',
    } as unknown as ConnectionResponse;

    expect(responseToFormValues(response, SMTP_FIELDS, REDIRECT)[AUTH_TYPE_FIELD]).toBeUndefined();
  });

  // A method the console was never compiled against must still round-trip.
  it('round-trips an unknown method and its fields', () => {
    const request = formValuesToRequest(
      {
        ...BASE_SMTP,
        [AUTH_TYPE_FIELD]: 'client_credentials',
        [authPropertyField('clientId')]: 'abc',
        [authPropertyField('tokenEndpoint')]: 'https://idp.acme.io/token',
      },
      SMTP_FIELDS,
      {mode: 'create'},
    ) as unknown as Record<string, unknown>;

    expect(request['authentication']).toEqual({
      properties: {clientId: 'abc', tokenEndpoint: 'https://idp.acme.io/token'},
      type: 'client_credentials',
    });
  });
});

describe('HTTP endpoint mapping', () => {
  const response = {
    id: 'h1',
    type: 'http',
    name: 'Risk Engine',
    url: 'https://risk.example.com/evaluate',
    timeoutMs: 5000,
    headers: [
      {name: 'X-Tenant', value: 'acme'},
      {name: 'Accept', value: 'application/json, text/plain'},
    ],
    authentication: {type: 'basic', properties: {username: 'svc', password: MASKED_SECRET}},
  } as unknown as ConnectionResponse;
  const valid = {name: 'Risk Engine', url: 'https://risk.example.com', timeoutMs: '10000'};
  const headers = (...pairs: [string, string][]): string =>
    encodeKeyValuePairs(
      pairs.map(([name, value]) => ({name, value})),
      'pairs',
    );

  it('round-trips the plain headers array, commas included, and keeps the stored password by omitting it', () => {
    const values = responseToFormValues(response, HTTP_FIELDS, REDIRECT);

    expect(values['timeoutMs']).toBe('5000');
    expect(values[AUTH_TYPE_FIELD]).toBe('basic');
    expect(values[authPropertyField('username')]).toBe('svc');
    expect(values[authPropertyField('password')]).toBe('');

    expect(formValuesToRequest(values, HTTP_FIELDS, {mode: 'edit'})).toEqual({
      name: 'Risk Engine',
      url: 'https://risk.example.com/evaluate',
      timeoutMs: 5000,
      headers: response.headers,
      authentication: {type: 'basic', properties: {username: 'svc'}},
    });
  });

  it('reads missing headers as empty and sends an empty list on edit so the last header can be removed', () => {
    const values = responseToFormValues({...response, headers: null}, HTTP_FIELDS, REDIRECT);

    expect(values['headers']).toBe('');
    expect(formValuesToRequest(values, HTTP_FIELDS, {mode: 'edit'})).toMatchObject({headers: []});
  });

  it('sends only the name and URL on create, leaving timeout and authentication to the server defaults', () => {
    const values = {...emptyFormValues(HTTP_FIELDS, REDIRECT), name: 'Risk Engine', url: 'https://risk.example.com'};
    const createFields = HTTP_FIELDS.filter((field) => field.visibility !== 'edit');

    expect(formValuesToRequest(values, createFields, {mode: 'create'})).toEqual({
      name: 'Risk Engine',
      url: 'https://risk.example.com',
    });
  });

  it('rejects a timeout outside 1 to 20000', () => {
    expect(validateConnectionForm({...valid, timeoutMs: '20001'}, HTTP_FIELDS, 'edit')).toEqual({
      timeoutMs: 'connections:validation.httpEndpointTimeoutRange',
    });
    expect(validateConnectionForm({...valid, timeoutMs: '20000'}, HTTP_FIELDS, 'edit')).toEqual({});
  });

  it('accepts an http URL when no authentication is set', () => {
    expect(validateConnectionForm({...valid, url: 'http://risk.example.com'}, HTTP_FIELDS, 'edit')).toEqual({});
    expect(
      validateConnectionForm({...valid, url: 'http://localhost:8090', [AUTH_TYPE_FIELD]: 'none'}, HTTP_FIELDS, 'edit'),
    ).toEqual({});
  });

  it('requires an https URL when authentication is set, loopback included', () => {
    for (const url of ['http://risk.example.com/evaluate', 'http://localhost:8090/evaluate', 'http://127.0.0.1']) {
      expect(validateConnectionForm({...valid, url, [AUTH_TYPE_FIELD]: 'basic'}, HTTP_FIELDS, 'edit')).toEqual({
        url: HTTPS_REQUIRED_WITH_AUTHENTICATION_ERROR,
      });
    }
    expect(validateConnectionForm({...valid, [AUTH_TYPE_FIELD]: 'basic'}, HTTP_FIELDS, 'edit')).toEqual({});
  });

  it('rejects a user name or password in the URL', () => {
    for (const url of ['https://svc@risk.example.com', 'https://svc:pw@risk.example.com/evaluate']) {
      expect(validateConnectionForm({...valid, url}, HTTP_FIELDS, 'create')).toEqual({
        url: 'connections:validation.httpEndpointUrlUserInfo',
      });
    }
  });

  it('rejects reserved header names in any letter case', () => {
    for (const name of ['Authorization', 'proxy-authorization', 'COOKIE', 'Host', 'content-length', 'Content-Type']) {
      expect(validateConnectionForm({...valid, headers: headers([name, 'x'])}, HTTP_FIELDS, 'edit')).toEqual({
        headers: 'connections:validation.httpEndpointHeaderReserved',
      });
    }
    for (const name of ['Transfer-Encoding', 'Connection', 'TE', 'Trailer', 'Upgrade', 'Keep-Alive', 'Expect']) {
      expect(validateConnectionForm({...valid, headers: headers([name, 'x'])}, HTTP_FIELDS, 'edit')['headers']).toBe(
        'connections:validation.httpEndpointHeaderReserved',
      );
    }
    expect(validateConnectionForm({...valid, headers: headers(['X-Tenant', 'acme'])}, HTTP_FIELDS, 'edit')).toEqual({});
  });

  it('rejects header names that commonly carry credentials in any letter case', () => {
    for (const name of ['X-Api-Key', 'api-key', 'X-AUTH-TOKEN', 'x-access-token']) {
      expect(validateConnectionForm({...valid, headers: headers([name, 'x'])}, HTTP_FIELDS, 'edit')).toEqual({
        headers: 'connections:validation.httpEndpointHeaderReserved',
      });
    }
  });

  it('flags a header row that has only a name or only a value', () => {
    for (const row of [
      ['X-Tenant', ''],
      ['', 'acme'],
    ] as [string, string][]) {
      expect(
        validateConnectionForm({...valid, headers: headers(['X-Region', 'eu'], row)}, HTTP_FIELDS, 'edit'),
      ).toEqual({headers: 'connections:validation.keyValuePair'});
    }
  });

  it('accepts header values that contain commas and colons', () => {
    expect(
      validateConnectionForm(
        {...valid, headers: headers(['Accept', 'application/json, text/plain: q=1'])},
        HTTP_FIELDS,
        'edit',
      ),
    ).toEqual({});
  });

  it('never sends a half-filled header row', () => {
    const values = {...valid, headers: headers(['X-Tenant', 'acme'], ['X-Region', ''])};

    expect(formValuesToRequest(values, HTTP_FIELDS, {mode: 'edit'})).toMatchObject({
      headers: [{name: 'X-Tenant', value: 'acme'}],
    });
  });

  it('rejects duplicate header names ignoring case', () => {
    expect(
      validateConnectionForm({...valid, headers: headers(['X-Tenant', 'a'], ['x-tenant', 'b'])}, HTTP_FIELDS, 'edit'),
    ).toEqual({
      headers: 'connections:validation.httpEndpointHeaderDuplicate',
    });
  });

  it('limits the name to 255 characters and the description to 500', () => {
    expect(validateConnectionForm({...valid, name: 'n'.repeat(255)}, HTTP_FIELDS, 'edit')).toEqual({});
    expect(validateConnectionForm({...valid, name: 'n'.repeat(256)}, HTTP_FIELDS, 'edit')).toEqual({
      name: 'connections:validation.httpEndpointNameLength',
    });
    expect(validateConnectionForm({...valid, description: 'd'.repeat(500)}, HTTP_FIELDS, 'edit')).toEqual({});
    expect(validateConnectionForm({...valid, description: 'd'.repeat(501)}, HTTP_FIELDS, 'edit')).toEqual({
      description: 'connections:validation.httpEndpointDescriptionLength',
    });
  });

  it('counts characters as the server does, so a name of 255 emoji is accepted', () => {
    expect(validateConnectionForm({...valid, name: '🚀'.repeat(255)}, HTTP_FIELDS, 'edit')).toEqual({});
  });
});

describe('HTTP endpoint authentication methods', () => {
  const methods: OutboundAuthMethod[] = [
    {type: 'none', displayName: 'None'},
    {
      type: 'basic',
      displayName: 'Username and Password',
      fields: [
        {key: 'username', type: 'string', required: true, displayName: 'Username'},
        {key: 'password', type: 'string', required: true, credential: true, displayName: 'Password'},
      ],
    },
    {
      type: 'bearer',
      displayName: 'Bearer token',
      fields: [{key: 'token', type: 'string', required: true, credential: true, displayName: 'Bearer token'}],
    },
    {
      type: 'api_key',
      displayName: 'API key',
      fields: [],
      additionalProperties: {key: '', type: 'string', required: true, credential: true, displayName: 'API key headers'},
    },
  ];
  const base = {name: 'Risk Engine', url: 'https://risk.example.com', timeoutMs: '10000', headers: ''};
  const apiKeyHeaders = (...pairs: [string, string][]): string =>
    encodeKeyValuePairs(
      pairs.map(([name, value]) => ({name, value})),
      'pairs',
    );

  it('sends a bearer token in the generic shape, leaving out values another method left behind', () => {
    const values = {
      ...base,
      [AUTH_TYPE_FIELD]: 'bearer',
      [authPropertyField('username')]: 'svc',
      [authPropertyField('token')]: 'tok-1',
    };

    expect(formValuesToRequest(values, HTTP_FIELDS, {mode: 'create', authenticationMethods: methods})).toMatchObject({
      authentication: {type: 'bearer', properties: {token: 'tok-1'}},
    });
  });

  it('sends API-key headers in the generic shape, each header as a property', () => {
    const values = {
      ...base,
      [AUTH_TYPE_FIELD]: 'api_key',
      [authPropertyField('username')]: 'svc',
      [AUTH_ADDITIONAL_PROPERTIES_FIELD]: apiKeyHeaders(['X-Api-Key', 'k-1'], ['X-Client', 'c, d']),
    };

    expect(formValuesToRequest(values, HTTP_FIELDS, {mode: 'create', authenticationMethods: methods})).toMatchObject({
      authentication: {type: 'api_key', properties: {'X-Api-Key': 'k-1', 'X-Client': 'c, d'}},
    });
  });

  it('restates stored API-key headers masked, so the server keeps their values', () => {
    const response = {
      id: 'h1',
      type: 'http',
      name: 'Risk Engine',
      url: 'https://risk.example.com',
      authentication: {type: 'api_key', properties: {'X-Api-Key': MASKED_SECRET}},
    } as unknown as ConnectionResponse;

    const values = responseToFormValues(response, HTTP_FIELDS, REDIRECT, methods);

    expect(values[authPropertyField('X-Api-Key')]).toBeUndefined();
    expect(formValuesToRequest(values, HTTP_FIELDS, {mode: 'edit', authenticationMethods: methods})).toMatchObject({
      authentication: {type: 'api_key', properties: {'X-Api-Key': MASKED_SECRET}},
    });
  });

  it('flags a half-filled or repeated API-key header and accepts a complete list', () => {
    const errorFor = (raw: string): string | undefined =>
      authenticationAdditionalPropertiesError(
        {[AUTH_TYPE_FIELD]: 'api_key', [AUTH_ADDITIONAL_PROPERTIES_FIELD]: raw},
        methods,
      );

    expect(errorFor(apiKeyHeaders(['X-Api-Key', '']))).toBe('connections:validation.keyValuePair');
    expect(errorFor(apiKeyHeaders(['', 'k-1']))).toBe('connections:validation.keyValuePair');
    expect(errorFor(apiKeyHeaders(['X-Api-Key', 'a'], ['x-api-key', 'b']))).toBe(
      'connections:validation.httpEndpointHeaderDuplicate',
    );
    expect(errorFor(apiKeyHeaders(['X-Api-Key', 'a']))).toBeUndefined();
    expect(
      authenticationAdditionalPropertiesError(
        {[AUTH_TYPE_FIELD]: 'bearer', [AUTH_ADDITIONAL_PROPERTIES_FIELD]: apiKeyHeaders(['X-Api-Key', ''])},
        methods,
      ),
    ).toBeUndefined();
  });
});

describe('urlOriginChanged', () => {
  it('ignores a change to the path, query or letter case of the host', () => {
    expect(urlOriginChanged('https://risk.example.com/evaluate', 'https://RISK.example.com/v2/evaluate?x=1')).toBe(
      false,
    );
  });

  it('treats an explicit default port as the same origin', () => {
    expect(urlOriginChanged('https://risk.example.com/evaluate', 'https://risk.example.com:443/evaluate')).toBe(false);
  });

  it('detects a change of scheme, host or port', () => {
    const stored = 'https://risk.example.com/evaluate';

    expect(urlOriginChanged(stored, 'http://risk.example.com/evaluate')).toBe(true);
    expect(urlOriginChanged(stored, 'https://collector.example.net/evaluate')).toBe(true);
    expect(urlOriginChanged(stored, 'https://risk.example.com:8443/evaluate')).toBe(true);
  });

  it('treats a URL that cannot be parsed as changed', () => {
    expect(urlOriginChanged('https://risk.example.com', 'risk.example.com')).toBe(true);
  });
});
