// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppliedConfiguration, AppliedResource} from './models';
import {isObject, valueAt} from './paths';

type Fields = Record<string, unknown>;

/** A request as the console's HTTP client makes it. */
export interface Request {
  url?: string;
  method?: string;
}

/** The pages of the configuration list resources thirty at a time when they do not say. */
const DEFAULT_LIMIT = 30;

const text = (value: unknown): string | undefined => (typeof value === 'string' ? value : undefined);

function fields(applied: AppliedResource): Fields {
  return isObject(applied.resource) ? applied.resource : {};
}

/** Which of a list's items a page asks for. */
function page<T>(items: T[], params: URLSearchParams): {items: T[]; total: number; offset: number} {
  const limit = Number(params.get('limit') ?? DEFAULT_LIMIT);
  const offset = Number(params.get('offset') ?? 0);
  return {items: items.slice(offset, offset + limit), total: items.length, offset};
}

/** A list as the configuration's own lists answer: a page of items under the collection's key. */
function listing<T>(key: string, items: T[], params: URLSearchParams): Fields {
  const shown = page(items, params);
  return {totalResults: shown.total, startIndex: shown.offset + 1, count: shown.items.length, [key]: shown.items};
}

/** What a user is called, as a read with display asked for says it. */
function displayOf(user: Fields): string | undefined {
  const attributes = isObject(user.attributes) ? user.attributes : {};
  return (
    text(user.display) ?? text(attributes.username) ?? text(attributes.email) ?? text(attributes.name) ?? text(user.id)
  );
}

/** The categories a connection's vendor serves, which the configuration's list filters by. */
function categoriesOf(type: unknown): string[] {
  switch (type) {
    case 'google':
    case 'github':
    case 'oidc':
    case 'oauth':
      return ['identity-provider'];
    case 'twilio':
    case 'vonage':
    case 'sms-gateway':
      return ['sms-provider'];
    case 'authzen-pdp':
      return ['authorization-pdp'];
    case 'email-smtp':
      return ['email-provider'];
    default:
      return [];
  }
}

/** Letters and digits alone, so `defaultResourceServer` and `default_resource_server` are one name. */
const plain = (name: string): string => name.replace(/[^a-z0-9]/gi, '').toLowerCase();

/**
 * Answers the reads the configuration's own pages make, from the configuration an environment runs,
 * so those pages show the environment as they show the configuration. Every resource is marked read
 * only, which is how those pages show a resource that cannot be changed. A read it cannot answer is
 * undefined, and a write is refused.
 */
export default class EnvironmentApi {
  private readonly byType = new Map<string, AppliedResource[]>();

  constructor(configuration: AppliedConfiguration | undefined) {
    configuration?.resources.forEach((applied: AppliedResource) => {
      this.byType.set(applied.resourceType, [...(this.byType.get(applied.resourceType) ?? []), applied]);
    });
  }

  private of(resourceType: string): Fields[] {
    return (this.byType.get(resourceType) ?? []).map((applied: AppliedResource) => ({
      ...fields(applied),
      id: text(fields(applied).id) ?? applied.id,
      isReadOnly: true,
    }));
  }

  private layers(name: string): Fields {
    const config = (this.byType.get('server_config') ?? []).find(
      (applied: AppliedResource) => plain(applied.id) === plain(name),
    );
    return isObject(config?.resource) ? config.resource : {readOnly: {}, writable: {}, merged: {}};
  }

  /**
   * Answers a request, or returns undefined for one it cannot answer, which the caller sends to the
   * server as it stands. A write throws, as the environment is only read here.
   */
  answer(request: Request, serverUrl: string): unknown {
    const url = new URL(request.url ?? '', 'https://environment.invalid');
    // The server's own path, if it has one, comes off; a bare origin has none to come off.
    const base = new URL(serverUrl, 'https://environment.invalid').pathname.replace(/\/+$/, '');
    const path = url.pathname.startsWith(`${base}/`) ? url.pathname.slice(base.length) : url.pathname;
    const params = url.searchParams;
    const method = (request.method ?? 'GET').toUpperCase();
    const own = this.answers(path.replace(/\/+$/, ''), params);
    if (method !== 'GET') {
      if (own === undefined) return undefined;
      throw Object.assign(new Error('An environment is only read here'), {response: {status: 405}});
    }
    return own;
  }

  private answers(path: string, params: URLSearchParams): unknown {
    switch (path) {
      case '/applications':
        return listing(
          'applications',
          this.of('application').map((application: Fields) => ({
            ...application,
            clientId: valueAt(application, 'oauth.clientId'),
          })),
          params,
        );
      case '/agents':
        return listing(
          'agents',
          this.of('agent').map((agent: Fields) => ({...agent, clientId: valueAt(agent, 'oauth.clientId')})),
          params,
        );
      case '/agent-types':
        return listing('types', this.of('agent_type'), params);
      case '/user-types':
        return listing('types', this.of('user_type'), params);
      case '/users':
        return listing(
          'users',
          this.of('user').map((user: Fields) => ({...user, display: displayOf(user)})),
          params,
        );
      case '/groups':
        return listing('groups', this.of('group'), params);
      case '/roles':
        return listing('roles', this.of('role'), params);
      case '/resource-servers':
        return listing('resourceServers', this.of('resource_server'), params);
      case '/organization-units':
        return listing(
          'organizationUnits',
          this.of('organization_unit').filter((unit: Fields) => !unit.parent),
          params,
        );
      case '/connections':
        return listing(
          'connections',
          this.of('connection').map((connection: Fields) => ({
            ...connection,
            categories: categoriesOf(connection.type),
          })),
          params,
        );
      case '/flows':
        return listing('flows', this.of('flow'), params);
      case '/design/themes':
        return listing(
          'themes',
          this.of('theme').map((theme: Fields) => ({
            ...theme,
            defaultColorScheme: valueAt(theme, 'theme.defaultColorScheme'),
            primaryColor: valueAt(theme, 'theme.colorSchemes.light.palette.primary.main'),
            // A theme's card does not open when the theme is read only, so it is left as it is.
            isReadOnly: undefined,
          })),
          params,
        );
      case '/design/layouts':
        return listing('layouts', this.of('layout'), params);
      case '/i18n/languages':
        return {
          languages: (this.byType.get('translation') ?? []).map(
            (applied: AppliedResource) => text(fields(applied).language) ?? applied.id,
          ),
        };
      case '/openid4vci/credential-configurations':
        return this.of('credential_configuration');
      case '/openid4vp/presentation-definitions':
        return this.of('presentation_definition');
      default:
        break;
    }
    const children = /^\/organization-units\/([^/]+)\/ous$/.exec(path);
    if (children) {
      const parent = decodeURIComponent(children[1]);
      return listing(
        'organizationUnits',
        this.of('organization_unit').filter((unit: Fields) => unit.parent === parent),
        params,
      );
    }
    const config = /^\/server-config\/([^/]+)$/.exec(path);
    if (config) {
      return this.layers(decodeURIComponent(config[1]));
    }
    return undefined;
  }
}
