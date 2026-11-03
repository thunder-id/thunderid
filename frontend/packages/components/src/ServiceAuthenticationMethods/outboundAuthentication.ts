// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AuthenticationMethods, type AuthenticationMethod} from './authentication-methods';
import {parseKeyValuePairs, serializeKeyValuePairs} from '../KeyValuePairsField/keyValuePairs';

/** The placeholder value the API returns for stored secrets. */
export const MASKED_SECRET = '******';

/**
 * How ThunderID authenticates itself on an outbound call. `type` selects the method and
 * `properties` carries that method's field values, so a method the console was not built
 * against still round-trips.
 *
 * A field the method marks as a credential is write-only: returned masked as "******", and
 * omitted on update to keep the stored value.
 */
export interface OutboundAuthentication {
  type: string;
  properties?: Record<string, string>;
}

/** The flat form values edited by {@link ServiceAuthenticationMethods}. */
export interface ServiceAuthenticationFormValues {
  authenticationScheme?: string;
  bearerToken?: string;
  basicUsername?: string;
  basicPassword?: string;
  httpHeaders?: string;
}

/** Validation errors of the service authentication form, keyed by field. */
export interface ServiceAuthenticationErrors {
  bearerToken?: string;
  basicUsername?: string;
  basicPassword?: string;
  httpHeaders?: string;
}

/**
 * Convert flat authentication form values into the structured outbound-authentication API contract.
 * A secret is sent exactly as entered, as the server stores it, so only the username is trimmed.
 */
export function outboundAuthenticationFromFormValues(values: ServiceAuthenticationFormValues): OutboundAuthentication {
  const scheme = (values.authenticationScheme as AuthenticationMethod | undefined) ?? AuthenticationMethods.NONE;
  if (scheme === AuthenticationMethods.BEARER) {
    return {type: 'bearer', properties: {token: values.bearerToken ?? ''}};
  }
  if (scheme === AuthenticationMethods.BASIC) {
    return {
      type: 'basic',
      properties: {
        username: (values.basicUsername ?? '').trim(),
        password: values.basicPassword ?? '',
      },
    };
  }
  if (scheme === AuthenticationMethods.API_KEY) {
    const headers = parseKeyValuePairs(values.httpHeaders ?? '');
    return {
      type: 'api_key',
      properties: Object.fromEntries(headers.map(({name, value}) => [name, value])),
    };
  }
  return {type: 'none'};
}

/** What is wrong with one API key header, in the order the server checks it. */
export type APIKeyHeaderProblem =
  | 'nameRequired'
  | 'invalidName'
  | 'reservedName'
  | 'duplicateName'
  | 'valueRequired'
  | 'valueReentry';

/** A header name is an HTTP token. */
const HEADER_NAME_PATTERN = /^[A-Za-z0-9!#$%&'*+\-.^_`|~]+$/;

/** Headers the HTTP client sets itself, which the server refuses as API key headers. */
const RESERVED_HEADER_NAMES = new Set([
  'content-type',
  'content-length',
  'host',
  'transfer-encoding',
  'connection',
  'te',
  'trailer',
  'upgrade',
  'proxy-connection',
  'keep-alive',
  'proxy-authorization',
]);

/**
 * Check serialized API key headers against the rules the server applies, returning the problem of
 * each header in order, or undefined for a header that is fine. A masked value keeps the stored
 * value, which is possible only for a header in `keptHeaders`, the serialized stored headers that
 * can still be kept.
 */
export function getAPIKeyHeaderProblems(raw: string, keptHeaders: string): (APIKeyHeaderProblem | undefined)[] {
  const keptNames = new Set(parseKeyValuePairs(keptHeaders).map(({name}) => name.toLowerCase()));
  const seen = new Set<string>();
  return parseKeyValuePairs(raw).map(({name, value}): APIKeyHeaderProblem | undefined => {
    const lowerName = name.toLowerCase();
    const duplicate = seen.has(lowerName);
    seen.add(lowerName);
    if (name === '') return 'nameRequired';
    if (!HEADER_NAME_PATTERN.test(name)) return 'invalidName';
    if (RESERVED_HEADER_NAMES.has(lowerName)) return 'reservedName';
    if (duplicate) return 'duplicateName';
    if (value === '') return 'valueRequired';
    if (value === MASKED_SECRET && !keptNames.has(lowerName)) return 'valueReentry';
    return undefined;
  });
}

/**
 * Flatten a response's outbound authentication into the form values. Secrets come back masked, so
 * the bearer token and the Basic password are blanked and shown as stored instead. API key headers
 * keep their masked values, so each stored header stays locked until it is replaced.
 */
export function outboundAuthenticationToFormValues(
  authentication: OutboundAuthentication | undefined,
): Required<ServiceAuthenticationFormValues> {
  const properties = authentication?.type === 'api_key' ? authentication.properties : undefined;
  const username: unknown = authentication?.properties?.['username'];
  return {
    authenticationScheme: authentication?.type?.toUpperCase() ?? AuthenticationMethods.NONE,
    bearerToken: '',
    basicUsername: typeof username === 'string' ? username : '',
    basicPassword: '',
    httpHeaders: serializeKeyValuePairs(Object.entries(properties ?? {}).map(([name, value]) => ({name, value}))),
  };
}

export interface ServiceAuthenticationStateOptions {
  /** Whether the user chose to replace the stored secret. */
  replacing: boolean;
  /**
   * Whether the endpoint the credential is sent to changed from the stored one. A stored credential
   * is kept only for the endpoint it was saved for, so a changed endpoint needs it entered again.
   */
  targetChanged: boolean;
  /** The message shown on a credential that has to be entered. */
  requiredMessage: string;
}

export interface ServiceAuthenticationState {
  selectedAuthenticationScheme: AuthenticationMethod;
  persistedAuthenticationScheme: string;
  /** Whether the selected method's credential has to be entered again for a changed endpoint. */
  requiresFreshCredentials: boolean;
  /** Whether a credential is stored for the selected method and can be kept. */
  hasStoredCredentialForSelectedScheme: boolean;
  /** Whether API key headers are stored for the selected method and can be kept. */
  hasStoredHTTPHeaders: boolean;
  /**
   * The stored API key headers, serialized with their masked values, or empty when none are stored.
   * Choosing API key again restores them.
   */
  storedHTTPHeaders: string;
  authenticationErrors: ServiceAuthenticationErrors;
  /** Whether every credential the selected method needs is entered or stored. */
  hasRequiredAuthenticationCredential: boolean;
  /** Whether a credential of the selected method is entered. */
  hasEnteredAuthenticationCredential: boolean;
  /**
   * Whether the authentication differs from the stored one, so it has to be sent. When it has not
   * changed it is omitted, and the server keeps the stored credential.
   */
  authenticationChanged: boolean;
}

/**
 * Derive what the service authentication form needs from its values and the stored outbound
 * authentication: which credentials are stored and can be kept, which have to be entered, and
 * whether the authentication changed.
 */
export function getServiceAuthenticationState(
  values: ServiceAuthenticationFormValues,
  persisted: OutboundAuthentication | undefined,
  {replacing, targetChanged, requiredMessage}: ServiceAuthenticationStateOptions,
): ServiceAuthenticationState {
  const selectedAuthenticationScheme =
    (values.authenticationScheme as AuthenticationMethod | undefined) ?? AuthenticationMethods.NONE;
  const persistedAuthenticationScheme = persisted?.type?.toUpperCase() ?? AuthenticationMethods.NONE;
  const requiresFreshCredentials = targetChanged && selectedAuthenticationScheme !== AuthenticationMethods.NONE;
  const hasStoredCredentialForSelectedScheme =
    !targetChanged &&
    selectedAuthenticationScheme === persistedAuthenticationScheme &&
    (selectedAuthenticationScheme === AuthenticationMethods.BEARER
      ? Boolean(persisted?.properties?.['token'])
      : selectedAuthenticationScheme === AuthenticationMethods.BASIC
        ? Boolean(persisted?.properties?.['username']) && Boolean(persisted?.properties?.['password'])
        : false);
  const storedHTTPHeaders =
    persistedAuthenticationScheme === AuthenticationMethods.API_KEY
      ? outboundAuthenticationToFormValues(persisted).httpHeaders
      : '';
  const hasStoredHTTPHeaders =
    !targetChanged && selectedAuthenticationScheme === AuthenticationMethods.API_KEY && storedHTTPHeaders !== '';
  // At least one header is needed, and each has to be one the server accepts. Stored headers kept
  // as they are pass, as their masked values keep the stored ones.
  const enteredHeaders = parseKeyValuePairs(values.httpHeaders ?? '');
  const headerProblems = getAPIKeyHeaderProblems(
    values.httpHeaders ?? '',
    hasStoredHTTPHeaders ? storedHTTPHeaders : '',
  );
  const authenticationErrors: ServiceAuthenticationErrors = {
    bearerToken:
      selectedAuthenticationScheme === AuthenticationMethods.BEARER &&
      (replacing || !hasStoredCredentialForSelectedScheme) &&
      ((values.bearerToken ?? '').trim() === '' || (values.bearerToken ?? '').trim() === MASKED_SECRET)
        ? requiredMessage
        : undefined,
    basicUsername:
      selectedAuthenticationScheme === AuthenticationMethods.BASIC &&
      (replacing || !hasStoredCredentialForSelectedScheme) &&
      (values.basicUsername ?? '').trim() === ''
        ? requiredMessage
        : undefined,
    basicPassword:
      selectedAuthenticationScheme === AuthenticationMethods.BASIC &&
      (replacing || !hasStoredCredentialForSelectedScheme) &&
      ((values.basicPassword ?? '').trim() === '' || (values.basicPassword ?? '').trim() === MASKED_SECRET)
        ? requiredMessage
        : undefined,
    httpHeaders:
      selectedAuthenticationScheme === AuthenticationMethods.API_KEY &&
      (enteredHeaders.length === 0 || headerProblems.some(Boolean))
        ? requiredMessage
        : undefined,
  };
  const hasRequiredAuthenticationCredential = Object.values(authenticationErrors).every((error) => !error);
  const hasEnteredAuthenticationCredential =
    selectedAuthenticationScheme === AuthenticationMethods.BEARER
      ? (values.bearerToken ?? '').trim() !== ''
      : selectedAuthenticationScheme === AuthenticationMethods.BASIC
        ? (values.basicUsername ?? '').trim() !== '' && (values.basicPassword ?? '').trim() !== ''
        : selectedAuthenticationScheme === AuthenticationMethods.API_KEY
          ? (values.httpHeaders ?? '').trim() !== ''
          : false;
  const authenticationChanged =
    selectedAuthenticationScheme !== persistedAuthenticationScheme ||
    ((selectedAuthenticationScheme === AuthenticationMethods.BEARER ||
      selectedAuthenticationScheme === AuthenticationMethods.BASIC) &&
      (replacing || hasEnteredAuthenticationCredential)) ||
    (selectedAuthenticationScheme === AuthenticationMethods.API_KEY && values.httpHeaders !== storedHTTPHeaders);

  return {
    selectedAuthenticationScheme,
    persistedAuthenticationScheme,
    requiresFreshCredentials,
    hasStoredCredentialForSelectedScheme,
    hasStoredHTTPHeaders,
    storedHTTPHeaders,
    authenticationErrors,
    hasRequiredAuthenticationCredential,
    hasEnteredAuthenticationCredential,
    authenticationChanged,
  };
}
