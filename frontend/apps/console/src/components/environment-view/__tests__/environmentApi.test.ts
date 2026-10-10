// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import EnvironmentApi from '../environmentApi';
import type {AppliedConfiguration} from '../models';

const SERVER = 'https://cp.example.com/api';

const configuration: AppliedConfiguration = {
  gatewayId: 'gw-2',
  version: 'abc',
  resources: [
    {
      resourceType: 'application',
      id: 'app-1',
      resource: {
        id: 'app-1',
        name: 'Orders',
        template: 'spa',
        inboundAuthConfig: [{type: 'oauth2', config: {clientId: 'var:APP_ORDERS_CLIENT_ID'}}],
      },
    },
    {resourceType: 'application', id: 'app-2', resource: {id: 'app-2', name: 'Billing'}},
    {resourceType: 'user', id: 'u-1', resource: {id: 'u-1', type: 'person', attributes: {email: 'a@b.test'}}},
    {resourceType: 'organization_unit', id: 'ou-1', resource: {id: 'ou-1', name: 'Root', handle: 'root'}},
    {resourceType: 'organization_unit', id: 'ou-2', resource: {id: 'ou-2', name: 'Sales', parent: 'ou-1'}},
    {resourceType: 'connection', id: 'c-1', resource: {id: 'c-1', name: 'Google', type: 'google'}},
    {resourceType: 'connection', id: 'c-2', resource: {id: 'c-2', name: 'SMS', type: 'twilio'}},
    {
      resourceType: 'theme',
      id: 't-1',
      resource: {
        id: 't-1',
        displayName: 'Dark',
        theme: {defaultColorScheme: 'dark', colorSchemes: {light: {palette: {primary: {main: '#123'}}}}},
      },
    },
    {resourceType: 'translation', id: 'fr', resource: {language: 'fr', translations: {}}},
    {resourceType: 'credential_configuration', id: 'vc-1', resource: {id: 'vc-1', handle: 'id-card'}},
    {
      resourceType: 'server_config',
      id: 'default_resource_server',
      resource: {readOnly: {}, writable: {resourceServerId: 'rs-1'}, merged: {resourceServerId: 'rs-1'}},
    },
  ],
};

const api = new EnvironmentApi(configuration);
const get = (path: string): unknown => api.answer({url: `${SERVER}${path}`, method: 'GET'}, SERVER);

describe('EnvironmentApi', () => {
  it('lists resources as the configuration lists them, read only, a page at a time', () => {
    expect(get('/applications?limit=30&offset=0')).toEqual({
      totalResults: 2,
      startIndex: 1,
      count: 2,
      applications: [
        expect.objectContaining({id: 'app-1', name: 'Orders', clientId: 'var:APP_ORDERS_CLIENT_ID', isReadOnly: true}),
        expect.objectContaining({id: 'app-2', isReadOnly: true}),
      ],
    });
    expect(get('/applications?limit=1&offset=1')).toMatchObject({
      totalResults: 2,
      startIndex: 2,
      count: 1,
      applications: [expect.objectContaining({id: 'app-2'})],
    });
  });

  it('names a user as a read with display does', () => {
    expect(get('/users?include=display')).toMatchObject({users: [expect.objectContaining({display: 'a@b.test'})]});
  });

  it('lists the root organization units, and the children of one', () => {
    expect(get('/organization-units?limit=30&offset=0')).toMatchObject({
      totalResults: 1,
      organizationUnits: [expect.objectContaining({id: 'ou-1'})],
    });
    expect(get('/organization-units/ou-1/ous?limit=30&offset=0')).toMatchObject({
      organizationUnits: [expect.objectContaining({id: 'ou-2'})],
    });
  });

  it('gives each connection the categories its vendor serves', () => {
    expect(get('/connections')).toMatchObject({
      connections: [
        expect.objectContaining({id: 'c-1', categories: ['identity-provider']}),
        expect.objectContaining({id: 'c-2', categories: ['sms-provider']}),
      ],
    });
  });

  it('lists themes with what their cards show, and leaves them openable', () => {
    expect(get('/design/themes?limit=30&offset=0')).toMatchObject({
      themes: [expect.objectContaining({id: 't-1', defaultColorScheme: 'dark', primaryColor: '#123'})],
    });
    expect((get('/design/themes') as {themes: {isReadOnly?: boolean}[]}).themes[0].isReadOnly).toBeUndefined();
  });

  it('lists the languages, and the credential configurations as a bare list', () => {
    expect(get('/i18n/languages')).toEqual({languages: ['fr']});
    expect(get('/openid4vci/credential-configurations')).toEqual([
      expect.objectContaining({id: 'vc-1', handle: 'id-card'}),
    ]);
    expect(get('/openid4vp/presentation-definitions')).toEqual([]);
  });

  it('answers a server config by name, however the name is written', () => {
    expect(get('/server-config/defaultResourceServer')).toMatchObject({merged: {resourceServerId: 'rs-1'}});
    expect(get('/server-config/cors')).toEqual({readOnly: {}, writable: {}, merged: {}});
  });

  it('leaves what it cannot answer to the server, and refuses a write', () => {
    expect(get('/connections/meta')).toBeUndefined();
    expect(api.answer({url: `${SERVER}/elsewhere/applications`, method: 'GET'}, SERVER)).toBeUndefined();
    expect(api.answer({url: `${SERVER}/connections/meta`, method: 'POST'}, SERVER)).toBeUndefined();
    expect(() => api.answer({url: `${SERVER}/groups`, method: 'POST'}, SERVER)).toThrow(
      'An environment is only read here',
    );
  });

  it('answers a server at a bare origin, and one at a path', () => {
    const origin = 'https://localhost:8090';
    expect(api.answer({url: `${origin}/i18n/languages`, method: 'GET'}, origin)).toEqual({languages: ['fr']});
    expect(api.answer({url: `${SERVER}/i18n/languages`, method: 'GET'}, SERVER)).toEqual({languages: ['fr']});
    expect(api.answer({url: `${origin}/api/i18n/languages`, method: 'GET'}, origin)).toBeUndefined();
  });

  it('runs nothing while nothing is applied', () => {
    expect(new EnvironmentApi(undefined).answer({url: `${SERVER}/groups`, method: 'GET'}, SERVER)).toEqual({
      totalResults: 0,
      startIndex: 1,
      count: 0,
      groups: [],
    });
  });
});
